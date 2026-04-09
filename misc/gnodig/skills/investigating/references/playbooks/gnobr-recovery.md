# gnobr Recovery Failures

Failure patterns after using gnobr (block rollback tool) to recover from a chain halt.
gnobr trims blockstore, patches state.db, wipes app state and WAL, resets priv_validator.
Pre-seeded from gnoland1 halt at height 352922 (2026-03-30).

## Entries

### Symptom: node panics with "wrong Block.Header.LastBlockID"
**Likely cause:** gnobr patched `LastBlockHeight` and `AppHash` but not `LastBlockID` in state.db. The state's `LastBlockID` still points to the deleted block (height N+1), while the blockstore's top block (height N) has a different ID.
**Check first:** `node_data_open` — look at `block_id_match`. If false, state and blockstore disagree on the last block's identity.
**Check second:** Compare `last_block_id` (from state) vs `blockstore_block_id`. If they differ, gnobr missed the BlockID patch.
**Check third:** Verify gnobr version — the `LastBlockID` fix was added in commit `e0981f9` (2026-04-02). Earlier versions only patched AppHash and height.
**Fix:** Re-run gnobr with fixed version. But see next entry — the guard condition may prevent the fix from applying.
**Code paths:** `contribs/gnobr/main.go` (state patching block), `tm2/pkg/bft/state/state.go` (LastBlockID field)
**MCP tools:** `node_data_open` (block_id_match check), `node_data_block` (verify blockstore block ID)
**Reference:** gnolang/gno#5410

### Symptom: re-running gnobr does not fix LastBlockID
**Likely cause:** gnobr's state patching is guarded by `if state.LastBlockHeight > targetHeight`. If a previous gnobr run already set the height to the target, the condition is false and no fields are patched — including the newly added LastBlockID fix.
**Check first:** `node_data_open` — if `latest_height == drop-after target` AND `block_id_match: false`, the guard is preventing the fix.
**Proof:** Re-fetch state.db after running the latest gnobr. If `block_id_match` is still false, the guard skipped the patch.
**Fix:** Change condition from `>` to `>=` in gnobr to make it idempotent. Re-running should always reconcile state fields against the blockstore.
**Code paths:** `contribs/gnobr/main.go:107` — `if state.LastBlockHeight > targetHeight`
**MCP tools:** `node_data_open` (before/after comparison)
**Reference:** gnolang/gno#5410

### Symptom: CONSENSUS FAILURE panic "not yet implemented" after gnobr recovery
**Likely cause:** The BlockID mismatch causes the node to hold votes for block ID X (from state) while peers send votes for block ID Y (from blockstore). TM2 interprets this as a `VoteConflictingVotesError` — equivocation detection. But the handler is unimplemented: the evidence pool code is commented out with `/* XXX */` and replaced with `panic("not yet implemented")`.
**Check first:** Search logs for `CONSENSUS FAILURE` with `err: "not yet implemented"`. The stack will point to `tryAddVote` in `state.go`.
**Check second:** Confirm BlockID mismatch via `node_data_open` — this is the precondition that triggers conflicting votes.
**Note:** This is a latent TM2 bug independent of gnobr. Any real equivocation event on the network would crash every node. gnobr's BlockID mismatch just happens to trigger it.
**Code paths:** `tm2/pkg/bft/consensus/state.go:1532-1542` (VoteConflictingVotesError handler)
**MCP tools:** `logs_search` (text="CONSENSUS FAILURE" or text="not yet implemented"), `node_data_open` (block_id_match)

### Symptom: WAL replay error "does not contain #ENDHEIGHT"
**Likely cause:** gnobr wipes the WAL directory, but if the WAL is recreated by a node start that then crashes (due to BlockID mismatch or signing errors), the new WAL will be incomplete — missing the `#ENDHEIGHT` marker for the rolled-back height.
**Check first:** Search logs for `cannot replay height N. WAL does not contain #ENDHEIGHT for N-1`.
**Check second:** The node logs this as an error but proceeds anyway ("Proceeding to start ConsensusState anyway"). It's a warning, not the root cause — look for the BlockID mismatch or signing errors that follow.
**Red herring:** This error looks scary but is a symptom of the incomplete recovery, not the cause. Fixing the BlockID mismatch and re-wiping the WAL resolves it.
**MCP tools:** `logs_search` (text="ENDHEIGHT"), `node_data_open` (check WAL status)

### Symptom: "Error signing proposal: step regression" after gnobr
**Likely cause:** gnobr resets `priv_validator_state.json` to height=0, round=0, step=0. On restart, the node replays to height N and tries to propose at N+1. But consensus state and signing state can get out of sync if the node crashed mid-recovery, leaving the signing step tracker expecting a higher step than the consensus round provides.
**Check first:** Search logs for `enterPropose: Error signing proposal` with `step regression`.
**Check second:** Check `priv_validator_state.json` — height/round/step should be 0/0/0 after a clean gnobr run, or match the current consensus height after a successful start.
**Fix:** Re-run gnobr (which resets priv_validator_state.json) and ensure the node starts cleanly in one go — no partial starts that corrupt the signing state.
**MCP tools:** `logs_search` (text="step regression"), `node_data_open`

## Recovery Checklist

When recovering a node with gnobr after a chain halt:

1. **Stop the node completely** before running gnobr
2. **Run gnobr** with `--drop-after N --app-hash <correct_hash>`
3. **Verify with `node_data_open`** — check `block_id_match: true`, AppHash matches, height matches
4. **If `block_id_match: false`** — gnobr has the guard bug. Patch the condition or manually fix state.db
5. **Start the node** with the fixed binary
6. **Check logs** for CONSENSUS FAILURE, signing errors, or WAL errors
7. **If node panics** — check which error, refer to entries above
