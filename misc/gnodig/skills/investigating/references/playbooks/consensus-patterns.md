# Consensus Patterns

Failure patterns for consensus halts, block production issues, and validator disagreements.
Pre-seeded from gnolang/gno issues/PRs and tendermint/cometbft ecosystem. Updated via investigation distillation.

## Entries

### Symptom: consensus halt with no obvious error
**Likely cause:** Gas-affecting cache cold/warm divergence after node restart. Nodes that have been running continuously have warm caches (typeCheckCache, store caches) while restarted nodes pay full store-read costs, producing different gas_used for identical transactions.
**Check first:** Did any validator restart recently? Compare gas_used for the halting block's transactions across validators using `node_data_block` on each node's data dir.
**Check second:** Check typeCheckCache population — look for missing leaf stdlibs (e.g. `time`, `regexp`, `math/rand`). These are packages reached as root calls, not as dependencies, and may not be cached on cold start.
**Red herring:** Path mismatches between nodes are usually cosmetic differences in import resolution, not the cause of gas divergence.
**Code paths:** `gnovm/pkg/gnolang/machine.go` (TypeCheckMemPackage), `vm.typeCheckCache`
**MCP tools:** `node_data_block` (compare gas across nodes), `node_compare` (diff at specific height), `node_data_state` (check store contents), `realm_source` (read deployed code to assess determinism risk), `realm_eval` (compare state across nodes)
**Reference:** gnolang/gno#5400, gnolang/gno#5401

### Symptom: validators disagree on block at a specific height, chain stops
**Likely cause:** Non-deterministic execution producing different appHash values. Common sources: cache state differences after restart, Go runtime behavior leaking into VM (append capacity, map iteration), platform-dependent constants.
**Check first:** Use `node_compare` with data dirs from 2+ validators at the halting height. Look at appHash and per-tx gas_used differences.
**Check second:** If gas_used differs, check if any node was restarted recently or compiled with a different Go version.
**Check third:** If the diverging tx is an `addpkg` or `m_call`, use `realm_source` to read the deployed code. Use `realm_eval` on multiple nodes with the same expression — different results = direct proof of state divergence.
**Red herring:** Don't chase log-level differences between nodes — focus on gas_used and appHash first.
**Code paths:** `gnovm/pkg/gnolang/machine.go`, `gno.land/pkg/sdk/vm/keeper.go`
**MCP tools:** `node_compare`, `node_data_block`, `node_data_wal`, `realm_source` (inspect deployed code), `realm_eval` (compare state across nodes)
**Reference:** gnolang/gno#4976, gnolang/gno#4983

### Symptom: WAL height > block store height on a stopped node
**Likely cause:** Node stopped mid-consensus — it wrote WAL entries for a height it never committed to the block store. This is normal for crash recovery but indicates the node was in the middle of proposing/voting when it went down.
**Check first:** Use `node_data_open` to see both heights. Then `node_data_wal` at the WAL height to see what consensus step it was in (proposal? prevote? precommit?).
**Check second:** Check if the WAL shows timeout messages — the node may have been stuck waiting for votes from unreachable peers.
**Red herring:** WAL height being 1 ahead of block store is normal (current round). Only concerning if the gap is larger or if the WAL shows repeated timeouts.
**Code paths:** `tm2/pkg/bft/consensus/wal.go`, `tm2/pkg/bft/state/`
**MCP tools:** `node_data_open`, `node_data_wal`
**Reference:** gnolang/gno#5394

### Symptom: node is up (RPC responds) but stopped making blocks, "CONSENSUS FAILURE" in logs
**Likely cause:** A panic in the consensus `receiveRoutine` was caught by `recover()`, which logs "CONSENSUS FAILURE!!!" and stops the consensus state machine. The node process stays alive, RPC keeps responding, ABCI app serves queries — but the node no longer participates in consensus. The panic is typically in `finalizeCommit` -> `ApplyBlock`.
**Check first:** `logs_search` for "CONSENSUS FAILURE" — this is the definitive signal. The log line includes the full panic stack trace.
**Check second:** `node_overview` will show height that never advances. Compare with `node_compare` against a healthy peer.
**Red herring:** Because RPC still responds, monitoring that only checks port or `/status` reports the node as healthy. The key is whether block height is advancing.
**Code paths:** `tm2/pkg/bft/consensus/state.go` (receiveRoutine with defer recover), `tm2/pkg/bft/state/execution.go` (ApplyBlock)
**MCP tools:** `logs_search` (text="CONSENSUS FAILURE"), `node_overview`, `node_compare`
**Reference:** cometbft/cometbft#4054

### Symptom: validators stuck voting nil, rounds keep incrementing, chain cannot make progress
**Likely cause:** Voting power distribution makes >2/3 impossible to achieve. Classic case: 2 validators with 66.67% and 33.33% power. If the 66.67% validator's proposal fails (e.g., remote signer refuses), the 33.33% validator alone cannot form >2/3. Both keep voting nil. Note: exactly 2/3 is NOT >2/3 — you need strictly more than 2/3.
**Check first:** `node_data_wal` at the stuck height — look for all-nil prevote patterns or vote splits where no block gets >2/3.
**Check second:** `node_overview` on multiple validators — check which are participating and their voting power. Calculate whether >2/3 is achievable with active validators.
**Red herring:** Operators focus on the most recent proposer. The real issue is that NO proposer can succeed because the math doesn't work with the current active set.
**Code paths:** `tm2/pkg/bft/consensus/state.go` (enterPrevote, enterPrecommit threshold checks), `tm2/pkg/bft/types/validator_set.go` (voting power math)
**MCP tools:** `node_data_wal`, `node_overview`, `node_compare`
**Reference:** cometbft/cometbft#4461

### Symptom: node stuck at same height after every restart, WAL replay produces "invalid proposal signature"
**Likely cause:** The proposer crashed after signing a proposal but before committing. On restart, WAL replay triggers a new round where the node is again proposer. The signer generates a NEW proposal (different hash), detects a double-sign attempt, and refuses. The node is permanently stuck: every restart replays the same WAL and hits the same signing refusal.
**Check first:** `node_data_wal` at the stuck height — look for "Replay: Proposal" followed by signature errors. The presence of both a local and conflicting peer proposal at same height/round is the tell.
**Check second:** Check signer logs for "refusing to sign" or "same block error" messages.
**Red herring:** Operators try deleting the WAL, which is dangerous. The real problem is the double-sign guard interacting with WAL replay. Clearing `priv_validator_state.json` height (carefully!) may be the fix.
**Code paths:** `tm2/pkg/bft/consensus/state.go` (catchupReplay), `tm2/pkg/bft/consensus/wal.go` (replay), `tm2/pkg/bft/privval/` (sign state)
**MCP tools:** `node_data_wal`, `logs_search` (text="invalid proposal signature")
**Reference:** cometbft/cometbft#1018

### Symptom: mempool/ABCI deadlock — node stops making blocks, no panic, no crash, goroutines blocked
**Likely cause:** Lock-ordering deadlock between mempool mutex and ABCI socket client mutex during large recheck. Mempool holds its lock, calls CheckTx on socket client. Client send buffer fills, blocking. Server sends response, receive goroutine needs client mutex (held by send). Callback needs mempool mutex (held by caller). Classic circular wait.
**Check first:** If node appears hung with no crash, get a goroutine dump (`kill -QUIT <pid>`). Look for cycle between mempool lock and ABCI client lock.
**Check second:** `logs_search` for the last "Committed state" entry. If mempool was large at that point, this is the likely cause.
**Red herring:** "Network partition" or "consensus timeout" — the node appears healthy from outside (RPC may respond). It just never proposes or votes because consensus is waiting on mempool.
**Code paths:** `tm2/pkg/bft/mempool/` (Update, recheck flow), `tm2/pkg/bft/abci/client/socket_client.go` (send/recv goroutines)
**MCP tools:** `logs_search`, `node_overview`, `node_compare`
**Reference:** tendermint/tendermint#9030
