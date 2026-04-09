# Store & State Bugs

Failure patterns in the Gno store, IAVL tree, realm persistence, and object tracking.
Pre-seeded from gnolang/gno issues/PRs.

## Entries

### Symptom: "unexpected object with id" panic during realm operation
**Likely cause:** Bug in `realm.go` object persistence tracking. The object ID system has issues with reference counting for deleted-then-recreated objects. Descendant reference count goes negative when objects are removed and re-added to AVL trees.
**Check first:** Check the stack trace for `realm.go` frames — specifically `MarkNewReal`, `MarkDirtyReal`, and recursive `decRefCount` paths. Repeated `realm.go:498` frames indicate the recursive descent through object graph.
**Check second:** Check if the operation involves a delete-then-recreate pattern across transaction boundaries (add key, delete key, re-add key in separate txs). Simple txtar tests won't reproduce this — it requires realm persistence across blocks.
**Red herring:** Don't look at the transaction content — the bug is in the persistence layer, not the transaction logic.
**Code paths:** `gnovm/pkg/gnolang/realm.go` (MarkNewReal, MarkDirtyReal, decRefCount)
**MCP tools:** `node_data_block` (find the failing tx), `node_data_tx` (decode the transaction), `logs_search` (search for "unexpected object")
**Reference:** gnolang/gno#1543, gnolang/gno#2266

### Symptom: "unexpected zero object id" in cross-realm operation
**Likely cause:** Object ID assignment happens in the owning realm, but cross-realm call paths don't properly propagate the ID. When one realm edits an object and another realm saves a reference, the ID may not be assigned.
**Check first:** Check if the transaction involves the `cross` keyword or cross-realm function calls.
**Check second:** Check if a reference to an object created in realm A is being persisted in realm B.
**Red herring:** The error looks like a nil pointer issue but it's actually an ownership tracking problem at the realm boundary.
**Code paths:** `gnovm/pkg/gnolang/realm.go`, cross-realm call handling in `machine.go`
**MCP tools:** `node_data_tx` (check for cross-realm calls), `logs_search` (search for "zero object id")
**Reference:** gnolang/gno#4818

### Symptom: state looks "impossible" — values that should have been rolled back are persisting
**Likely cause:** The Gno store cache layer doesn't handle rollback correctly. Unlike tm2's IAVL store (which supports begin/commit/rollback), the Gno `defaultStore` was historically a flat cache mutated in-place. Failed transactions could leave dirty cache entries.
**Check first:** Check if the impossible state appeared after a failed transaction. Use `node_data_block` to find txs that failed (success=false) near the problematic height.
**Check second:** Check if the node is running a version with `TransactionStore` support (gnolang/gno#2319). Older versions don't have store transactionality.
**Red herring:** Don't blame tm2's IAVL layer — it handles rollback correctly. The bug is in the Gno cache layer sitting on top of it.
**Code paths:** `gnovm/pkg/gnolang/store.go` (TransactionStore, BeginTransaction), `gno.land/pkg/sdk/vm/keeper.go` (BeginTxHook, EndTxHook)
**MCP tools:** `node_data_block`, `node_data_state` (compare state at height-1 vs height)
**Reference:** gnolang/gno#2319

### Symptom: state inconsistent after error return from realm function
**Likely cause:** Prior to the fix, only `panic` triggered state revert in GnoVM. Error returns were treated as successful execution with an error value — state changes persisted. This is counterintuitive for anyone from Ethereum/Cosmos where any failing tx reverts.
**Check first:** Check whether the realm function returned an error vs panicked. Use `node_data_tx` to see if the tx shows success=true despite the function returning an error.
**Check second:** Check the Gno version — this was a known design gap fixed in later versions.
**Red herring:** Don't assume all failed-looking transactions roll back state. Only panics guarantee rollback.
**Code paths:** `gnovm/pkg/gnolang/machine.go` (execution, error handling)
**MCP tools:** `node_data_tx`, `node_data_state` (check state before/after the tx)
**Reference:** gnolang/gno#1864

### Symptom: node panics on startup with "PubKey does not match Signer address"
**Likely cause:** Genesis transactions were signed with keys that don't match what the ante handler now expects. Usually caused by a commit that changed signing requirements (e.g., auth module update) while the genesis.json still contains old-format signatures.
**Check first:** Check if the genesis.json is bundled (like `-lazy` mode) and if there were recent changes to `sdk/auth/ante.go` or `sdk/auth/keeper.go`.
**Check second:** Try bisecting recent commits to find when the signing requirement changed.
**Red herring:** The error message names a specific address (e.g., `g1manfred...`) which makes it look like a key management issue. It's not — it's a code-level ante handler change.
**Code paths:** `gno.land/pkg/sdk/auth/ante.go`, genesis loading in `gno.land/cmd/gnoland/`
**MCP tools:** `chain_query` with method="genesis" (if node starts), `node_data_open` (check chain_id)
**Reference:** gnolang/gno#4476
