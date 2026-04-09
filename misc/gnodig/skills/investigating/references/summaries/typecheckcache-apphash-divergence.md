# Investigation Summary: typeCheckCache AppHash Divergence (gnoland1 + test12, 2026-04)

## Symptom

Chain halts immediately after an `addpkg` transaction. Validators split into two groups, each computing a different AppHash for the same block. Neither group has 2/3+ votes. Consensus cycles through rounds indefinitely with no progress.

Observed on gnoland1 (height 352923, 12 validators, 9-vs-2 split) and test12 (height 234889, 7 validators, 4-vs-3 split).

## Classification

Consensus halt → AppHash divergence → Non-deterministic gas computation → typeCheckCache warm/cold state

## Diagnostic Path

1. `node_doctor` on RPC → `chain_halted` finding. Last block was days ago, consensus stuck in prevote/precommit timeout cycle.
2. `node_overview` on multiple nodes → all at same blockstore height, same block hash for last committed block. Consensus shows `valid_block_hash` set but can't finalize.
3. `node_data_open` on two nodes from different validator groups → same `blockstore_app_hash` (consensus-committed), different `state_app_hash` (locally computed). This confirms the divergence is in execution, not in block propagation.
4. `node_compare` at the divergence block → **key finding:** same transaction, wildly different `gas_used`, opposite `success` values. On test12: 18.2M gas (fail) vs 13M gas (pass) for the same `addpkg` — a 40% gas difference.
5. `node_data_wal` at halt height → vote split visible. Two groups of validators voting for different block hashes, cycling through rounds. Neither reaches 2/3+.
6. `node_data_block` at trigger block → `addpkg` transaction with gas near the limit. The gas margin was tight enough that the warm/cold cache difference crossed the success/failure boundary.
7. `logs_search` for `enterPrevote` and `Added to prevote` → confirmed which validators are voting and which are silent. Matched the WAL vote split.

## Root Cause

The GnoVM has a `typeCheckCache` that caches type-checking results during package loading (`addpkg`). Validators with a **warm cache** (running continuously, having processed prior addpkg operations) consume less gas for type-checking. Validators with a **cold cache** (recently restarted) perform full type-checks and consume more gas.

This cache state depends on runtime history (which packages were loaded, when the node last restarted), not on-chain state. It is therefore non-deterministic across validators.

When an `addpkg` transaction has gas near the limit, the warm/cold difference can cross the success/failure boundary: the same transaction succeeds on warm-cache nodes and fails with `OutOfGasError` on cold-cache nodes. The resulting state is completely different (package deployed vs not deployed, different balances, different storage deposits), producing a large AppHash divergence.

Related: gnolang/gno#5400, gnolang/gno#5401

## Key Indicators

- `addpkg` transaction in the block immediately before the halt
- `OutOfGasError` at `SetTypePerByte` location (gas consumed during type serialization)
- Gas usage close to gas limit (tight margin)
- `node_compare` shows same tx hash with different `gas_used` and different `success`
- Vote split in WAL correlates with which validators were restarted recently
- Chain was healthy (empty blocks, normal cadence) immediately before the trigger tx

## Recovery

1. **gnobr** to roll back all validators to a height before the divergence block, with the correct AppHash (from the block header at height+1)
2. **Fixed binary** with typeCheckCache populated on all initialization paths
3. **Coordinated restart** — all validators must restart simultaneously from the same state

**Blockers encountered:**
- gnobr crashed with `amino: unrecognized concrete type tm.StorageDepositEvent` — missing blank import of `gnovm/stdlibs/chain` package. Fix: add `_ "github.com/gnolang/gno/gnovm/stdlibs/chain"` to gnobr imports.
- Wrong `--app-hash` used (off-by-one block). The correct hash for `--drop-after N` is the `app_hash` from block N+1's header (state after executing block N).
- gnobr's `isLockError` heuristic misclassified PebbleDB corruption (truncated SST files from rsyncing a live DB) as "database locked."

## Lessons

- `node_compare` is the fastest way to confirm divergence — shows exact gas/success diff in one call
- Check `gas_used` difference, not just success/failure — same tx can have wildly different gas across nodes
- When the gas margin is tight (gas_used close to gas_wanted), typeCheckCache divergence crosses the success/failure boundary — this is the worst-case scenario
- gnobr requires all amino types registered that appear in ABCI responses
- The correct `--app-hash` for `--drop-after N` is from block N+1 header, not block N
- Don't rsync PebbleDB from a running node — results in corrupt SST files
