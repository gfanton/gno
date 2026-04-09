# P2P & Topology

Failure patterns in peer connectivity, node discovery, sentry node setup, and network topology.
Pre-seeded from gnolang/gno issues/PRs and tendermint/cometbft ecosystem.

## Entries

### Symptom: sentry nodes lagging behind validator, "Commit is for a block we don't know about"
**Likely cause:** In a single-validator setup (100% voting power), the validator self-commits blocks faster than it can gossip the block data to sentries. Sentries receive the commit message before the block proposal and can't finalize.
**Check first:** Check if the validator has 100% voting power (single-validator devnet). Use `node_data_open` or `node_overview` to see the validator set.
**Check second:** Check sentry logs for "Commit is for a block we don't know about" — this is logged at INFO level (misleadingly, not ERROR).
**Red herring:** Multiple-validator setups don't exhibit this because consensus is slower, giving gossip time to deliver blocks. Don't try to reproduce with multiple validators.
**Code paths:** `tm2/pkg/bft/consensus/state.go`, `tm2/pkg/bft/consensus/reactor.go`
**MCP tools:** `logs_search` (text="Commit is for a block we don't know"), `node_overview` (check validator count)
**Reference:** gnolang/gno#2430

### Symptom: node has no peers despite seeds being configured
**Likely cause:** `config.P2P.Seeds` is defined in tm2's config but **never consumed** — the seed-specific dialing logic was removed when tm2 rewrote the P2P layer. Only `persistent_peers` actually works.
**Check first:** Check the node's config for `persistent_peers` vs `seeds`. If only `seeds` is set, that's the problem.
**Check second:** Use `node_overview` to check `num_peers` — it will be 0 if only seeds are configured.
**Red herring:** The `seeds` config field exists and the docs say to set it, so operators assume it works. The field's presence is the red herring itself.
**Code paths:** `tm2/pkg/bft/config/config.go` (P2P.Seeds — defined but unused)
**MCP tools:** `node_overview` (num_peers), `chain_query` with status
**Reference:** gnolang/gno#5340

### Symptom: peer count drops to 0, all reconnections rejected as "duplicate ID"
**Likely cause:** Race condition in `Switch.addPeer()`: MConnection is started before the peer is added to PeerSet. If MConnection receives a consensus message before `ConsensusReactor.AddPeer()` sets peer state, the reactor panics with "no state". `StopPeerForError` runs, but `Remove()` is a no-op since peer was never in PeerSet. Then `addPeer()` continues and adds the (dead) peer to PeerSet. All future connections from that peer ID are rejected as duplicates.
**Check first:** `logs_search` for the sequence: "Starting Peer" -> "Stopping peer for error: has no state" -> "Added peer" (note reversed order).
**Check second:** `node_overview` — if peer count is 0 and the node keeps logging "duplicate ID", this is the bug. A restart clears it.
**Red herring:** "Network partition" or "firewall blocking" — the real issue is internal PeerSet state corruption, not network. A restart fixes it.
**Code paths:** `tm2/pkg/p2p/switch.go` (addPeer, startInitPeer, stopAndRemovePeer), `tm2/pkg/p2p/set.go` (Add, Remove)
**MCP tools:** `logs_search`, `node_overview`
**Reference:** tendermint/tendermint#3304

### Symptom: slow peer stalls consensus — round times spike to 10+ seconds sporadically
**Likely cause:** MConnection send/receive rate limiter asymmetry. The send side can burst up to 10x the configured rate, while receive is strict. Under high load, the rate limiter starts sleeping to compensate, blocking ALL channels — including the critical consensus channel. Votes and proposals get delayed, causing timeouts. The workaround is setting `p2p.recv_rate` to 10x `p2p.send_rate`.
**Check first:** `logs_search` for consensus timeout messages that correlate with high network activity (many txs, large blocks).
**Check second:** Check p2p config: if `send_rate` and `recv_rate` are equal (default), the asymmetry bug is active.
**Red herring:** Operators increase timeout values, which masks the symptom. Others add more peers, which actually makes it worse by multiplying the rate limiter load.
**Code paths:** `tm2/pkg/p2p/conn/connection.go` (sendRoutine/recvRoutine, flow.Monitor interaction)
**MCP tools:** `node_overview`, `logs_search` (text="Timed out"), `node_compare`
**Reference:** cometbft/cometbft#3864

### Symptom: all peers lost, "Won't start a peer — switch is not running"
**Likely cause:** The P2P switch enters a stopped state while the node process is still alive. A slow ABCI response or race condition causes the switch to mark itself as not running. Peers disconnect because the node stops responding to P2P messages. The PEX reactor cannot re-establish connections because the switch is stopped.
**Check first:** `node_overview` — peer count at 0 but process alive.
**Check second:** `logs_search` for "switch is not running" or "Won't start a peer". Navigate to the timestamp where peers dropped to find what triggered the switch shutdown.
**Red herring:** Operators assume firewall or network issue. The real cause is internal — the switch goroutine exited due to an unrecoverable error.
**Code paths:** `tm2/pkg/p2p/switch.go` (OnStop, isRunning checks), PEX reactor ensurePeers loop
**MCP tools:** `node_overview`, `logs_search`, `logs_navigate`
**Reference:** cometbft/cometbft#521, cometbft/cometbft#815

### Symptom: block sync node cannot catch up, "peer is not sending us data fast enough"
**Likely cause:** Block sync pool has a minimum data rate threshold (default 128 KB/s). When replaying consecutive empty blocks, byte throughput is low (small blocks, mostly overhead) even though blocks/second is high. The rate limiter measures bytes/second, not blocks/second. The peer gets disconnected, waits ~2 min to reconnect, catches up briefly, then hits threshold again. With many empty blocks, the node can never catch up.
**Check first:** `logs_search` for "not sending us data fast enough" and the associated rate values.
**Check second:** `node_data_block` at the heights being synced — if blocks have 0 txs, data rate will be low.
**Red herring:** Operators think network is slow or peer is overloaded. The real issue is the rate threshold being inappropriate for small blocks.
**Code paths:** `tm2/pkg/bft/blockchain/pool.go` (minRecvRate check), `tm2/pkg/p2p/conn/connection.go` (flow.Monitor)
**MCP tools:** `logs_search`, `node_data_block`, `node_overview`
**Reference:** cometbft/cometbft#5135
