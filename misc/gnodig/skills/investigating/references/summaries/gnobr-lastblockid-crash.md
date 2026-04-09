# Investigation Summary: gnobr LastBlockID Crash (gnoland1, 2026-04)

## Symptom

After running gnobr to recover from a chain halt, the node replays blocks successfully but panics on entering consensus with:
```
CONSENSUS FAILURE!!!
+2/3 committed an invalid block: wrong Block.Header.LastBlockID.
Expected <stale-id> got <real-id>
```

The node completed full block replay (hours of work) only to crash immediately when it tried to participate in consensus.

## Classification

Recovery failure → gnobr state corruption → incomplete state.db patching → ValidateBlock panic

## Diagnostic Path

1. `node_data_open` on the crashed node → `block_id_match: false`. This is the first signal — state.LastBlockID doesn't match the actual BlockID of the block at that height in the blockstore.
2. Compared `last_block_id` (from state.db) vs `blockstore_block_id` (from blockstore) — different values. The state.db value was the BlockID of the now-deleted block (one height above the gnobr target).
3. Read gnobr source code (`contribs/gnobr/main.go`) — confirmed it patches only `LastBlockHeight` and `AppHash`, not `LastBlockID` or other state fields.
4. `logs_search` for `CONSENSUS FAILURE` → found the exact panic with the mismatched BlockIDs.
5. Traced the crash path: `finalizeCommit` → `ValidateBlock` → `block.LastBlockID != state.LastBlockID` → panic at `state.go:1324`.

## Root Cause

gnobr patches only 2 of ~12 fields in the TM2 `State` struct:

| Field | Patched? | Checked by ValidateBlock? |
|-------|----------|--------------------------|
| `LastBlockHeight` | Yes | — |
| `AppHash` | Yes | — |
| `LastBlockID` | **No** | **Yes** |
| `LastResultsHash` | **No** | **Yes** |
| `LastBlockTime` | **No** | Yes |
| `LastBlockTotalTx` | **No** | — |

When gnobr sets `LastBlockHeight = N` but leaves `LastBlockID` untouched, the state retains the BlockID of the deleted block at height N+1. When the node enters consensus and receives block N+1, `ValidateBlock` compares the real previous block's ID against the stale state value — mismatch → panic.

## Key Indicators

- `block_id_match: false` in `node_data_open` output — this is the definitive signal
- State `last_block_id` doesn't match `blockstore_block_id`
- Node completed full replay successfully but crashes on entering consensus
- Panic message contains "wrong Block.Header.LastBlockID" with two different hashes

## Recovery

Fix gnobr to reconstruct all state fields from the block at `targetHeight`:
```go
targetMeta := bs.LoadBlockMeta(targetHeight)
state.LastBlockID      = targetMeta.BlockID
state.LastBlockTime    = targetMeta.Header.Time
state.LastBlockTotalTx = targetMeta.Header.TotalTxs
```

Also load ABCI responses for `LastResultsHash` — but this requires all amino types to be registered (see typecheckcache-apphash-divergence summary for the amino import fix).

## Lessons

- `node_data_open`'s `block_id_match` check catches gnobr state corruption in one call — always check this first after gnobr runs
- gnobr state patching must be comprehensive — any field checked by `ValidateBlock` that isn't patched will cause a panic
- The crash happens AFTER full replay (hours of work), making it especially painful — validators must re-gnobr and replay again
- Two sequential gnobr bugs can compound: wrong app hash (attempt 1) → fix hash but still crash on LastBlockID (attempt 2)
