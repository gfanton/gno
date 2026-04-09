# VM Execution

Failure patterns in GnoVM execution, DoS vectors, and transaction processing.
Pre-seeded from gnolang/gno issues/PRs.

## Entries

### Symptom: node crashes processing a specific transaction
**Likely cause:** Deeply nested recursive data structures that exhaust CPU or memory during processing (printing, serialization, persistence). A simple Gno program like `var x interface{}; for { x = [1]interface{}{x} }` can crash the node.
**Check first:** Use `node_data_block` to identify the crashing transaction, then `node_data_tx` to decode it. Look for realm calls that might construct recursive or deeply-nested data.
**Check second:** Check if gas metering covers the processing path that crashed. Gas must account for ALL processing — not just VM opcodes but also printing, serialization, and persistence of nested structures.
**Red herring:** The crash may look like an OOM or stack overflow, but the root cause is missing depth limits in a specific processing path.
**Code paths:** `gnovm/pkg/gnolang/machine.go` (execution), `gnovm/pkg/gnolang/realm.go` (persistence)
**MCP tools:** `node_data_block`, `node_data_tx`, `logs_search` (search for "panic" or "runtime error")
**Reference:** gnolang/gno#3471

### Symptom: flaky consensus test "WAL did not panic for N seconds"
**Likely cause:** Test uses `crashingWAL` with an incremental crash strategy — iteration N allows N-1 WAL writes before crashing. Under CI CPU pressure, goroutine scheduling delays accumulate. ~300ms delay per write at iteration 30 exceeds hardcoded timeouts.
**Check first:** Check if the test only fails in CI, never locally. This is the hallmark of timing-dependent tests on shared runners.
**Check second:** Check the timeout value in the test — it's likely hardcoded for ideal CPU, not shared CI runners.
**Red herring:** Don't chase consensus logic bugs — the test infrastructure (timing) is the problem, not the code being tested.
**Code paths:** `tm2/pkg/bft/consensus/wal_test.go`
**MCP tools:** Not applicable (CI issue, not chain issue)
**Reference:** gnolang/gno#5394
