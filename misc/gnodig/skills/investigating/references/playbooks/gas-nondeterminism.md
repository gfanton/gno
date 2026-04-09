# Gas Non-Determinism

Failure patterns where identical transactions produce different gas consumption across nodes.
This is the #1 cause of chain halts in gno.land. Pre-seeded from gnolang/gno issues/PRs.

## Entries

### Symptom: same transaction yields different gas after node restart
**Likely cause:** Store/cache state differs between freshly started node and continuously running node. The GnoVM's in-memory caches (typeCheckCache, cacheObjects, cacheTypes) are populated lazily — a warm node has more cached entries, skipping store reads that cost gas.
**Check first:** Reproduce by comparing gas_used for the same tx on a node before and after restart. Use `node_data_block` at the same height on both nodes.
**Check second:** Check `typeCheckCache` — leaf stdlibs (packages reached as root calls, not dependencies) may be missing from the cache on cold start. The cache is populated via `ImportFrom` which only caches dependencies, not the root package.
**Check third:** If the tx is `addpkg`, use `realm_source` to read the deployed code and assess whether type-checking complexity could trigger cache-dependent gas differences. Use `node_data_state` at height-1 vs height to confirm the package was created in that block.
**Red herring:** Small gas differences (~6k) seem negligible but consensus requires exact match. Don't dismiss small deltas.
**Code paths:** `gnovm/pkg/gnolang/machine.go` (TypeCheckMemPackage, typeCheckCache)
**MCP tools:** `node_data_block` (compare gas), `node_compare`, `realm_source` (inspect deployed code), `node_data_state` (verify state change)
**Reference:** gnolang/gno#4983, gnolang/gno#5400

### Symptom: nodes compiled with different Go versions disagree on execution
**Likely cause:** GnoVM delegates some operations to the host Go runtime. `[]rune` conversion uses Go's `append`, which has version-dependent growth behavior. `cap([]rune(s))` can differ between Go versions.
**Check first:** Verify all validators are running the same Go version (`go version` on each node).
**Check second:** Check for any code path that uses `append` for capacity-sensitive operations — the capacity growth factor changed between Go 1.21 and 1.22.
**Red herring:** The values themselves will be correct — only the slice capacity differs. This makes the bug very hard to spot in output comparison.
**Code paths:** `gnovm/pkg/gnolang/op_expressions.go` (string-to-rune conversion)
**MCP tools:** `node_compare`, `chain_query` (compare node versions via status)
**Reference:** gnolang/gno#5183

### Symptom: platform-dependent values in VM execution
**Likely cause:** Use of Go's architecture-dependent constants in the VM. `math.MinInt` / `math.MaxInt` differ between 32-bit and 64-bit builds. These leak into float-to-integer conversion boundaries.
**Check first:** Grep the VM source for `math.MinInt`, `math.MaxInt`, `unsafe.Sizeof` — any use of platform-dependent values is a determinism bug.
**Check second:** Verify all validators are running the same architecture (should all be amd64).
**Red herring:** This is rare in practice since most nodes run amd64, but can surface in CI or development environments mixing architectures.
**Code paths:** `gnovm/pkg/gnolang/op_expressions.go`
**MCP tools:** `chain_query` with method="status" to check node info
**Reference:** gnolang/gno#3288
