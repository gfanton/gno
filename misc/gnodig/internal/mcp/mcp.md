# gnodig MCP Tool Descriptions

## instructions

gnodig investigates gno.land chain incidents. Two investigation modes:

FIRST CALL (recommended):
- node_doctor(target) — comprehensive health check. Returns findings + context summary in one call.
  Replaces the manual orient phase. If findings are clear, skip to domain expert dispatch.

LIVE NODE (via RPC):
1. node_overview(target) — is the node healthy? What height? Peers? Consensus?
2. peer_consensus(target) — what are peers doing in consensus? Heights, votes, alignment?
3. block_inspect(target, height) — what happened at this height?
4. account_info(target, address) — who is this account? Balance, sequence?
5. realm_inspect(target, pkg_path) — what does this realm do? Functions, files?
6. realm_eval(target, pkg_path, expression) — query realm state or Render output
7. realm_source(target, pkg_path, file) — read specific source file
8. logs_search(target, text) — find the smoking gun in logs
9. logs_navigate(target, time) — zoom into the exact moment

OFFLINE NODE (from data directory):
1. node_data_open(target) — orient: height, validators, appHash, WAL status, state consistency checks
2. node_data_block(target, height) — block header + identity (BlockID, hashes) + decoded tx summaries
3. node_data_tx(target, hash) — full tx payload when you need message details
4. node_data_wal(target, height) — consensus digest: who voted for what?
5. node_data_wal(target, height, mode=raw, round=0) — drill into specific round
6. node_data_state(target, path, height) — on-chain state at height
7. node_compare(targets, height) — diff two+ nodes at same height

GENESIS:
- genesis_info(target) — chain metadata, validators, consensus params (cached)
- genesis_info(target, mode=balance, address=...) — genesis balance lookup

ESCAPE HATCH:
- chain_query(target, method, params) — raw RPC for anything else

Start with node_overview (live) or node_data_open (offline) to orient. Then drill into specific heights, time ranges, or state.

WHEN A TX IS INVOLVED (addpkg, m_call):
1. node_data_block or block_inspect to identify the tx
2. node_data_tx to decode full payload (sender, args, pkg_path)
3. realm_source(target, pkg_path, file) — read the deployed code. Assess determinism risk.
4. realm_eval(target, pkg_path, expression) — check live state. Compare across nodes for divergence proof.
5. node_data_state(target, path, height) — check state before/after the tx to confirm what changed.
If two nodes return different realm_eval results for the same expression, that's direct proof of state divergence.

PERFORMANCE NOTES:
- Log indexing takes 1-3 minutes on first call for large files (200GB+), then near-instant.
- WAL summary mode is the default — no need to specify mode=summary.
- node_data_tx hash lookup scans up to 10,000 blocks backward — prefer height+index when you know the block.
- node_data_state requires IAVL versions retained at the requested height — pruned heights return errors.

TARGET PARAMETER:
- RPC: http://node:26657
- Logs: file:///path/to/node.log (or bare path). Works with raw JSONL, syslog-prefixed JSON (journald), and docker log formats.
- Data dir: /path/to/gnoland-data (directory with db/, wal/, config/)

USER-PROVIDED LOG FILES:
When a user provides log files (extracted tarballs, docker logs, copied files), use logs_summary and logs_search with `file:///path/to/extracted/file` as the target. These are the same log tools used for live investigation — always prefer them over manual grep/bash.

## node_overview

Get a comprehensive overview of a running gno.land node in a single call. Returns sync state (catching_up, latest_block_height, latest_block_time), peer connectivity (num_peers, listeners), consensus progress (round, step, proposer), and mempool status (num_unconfirmed_txs). Use as the first call when investigating a live node — gives enough context to decide what to dig into next. Does NOT return block contents or transaction details — use block_inspect for that.

Required: `target` — RPC endpoint URL (e.g. "http://node:26657").

## block_inspect

Get complete information about a block at a specific height: header (proposer, time, app hash), transaction results (success/failure, gas, events), and the validator set. Returns everything needed to understand what happened at this height in one call. Height 0 means latest block. Does NOT return decoded transaction content — use chain_query with method="tx" for individual transaction details via RPC.

Parameters:
- `target` (required) — RPC endpoint URL (e.g. "http://node:26657")
- `height` — block height (0 = latest)

## logs_search

Search structured JSON log files by text substring, field match, log level, module, and time range. Returns matching log entries, each with the full JSON line, byte offset, parsed timestamp (nanoseconds), and level. Builds a block index on first call for fast subsequent queries. Maximum 200 results per call. Use `module` or `exclude_module` to filter noisy sources (e.g. exclude p2p dial failures). Use `deduplicate=true` to group identical messages by template — when enabled, returns a DedupResult with groups (template, count, first_seen, last_seen, sample) instead of individual entries. Does NOT return surrounding context lines — use logs_navigate to read around a specific offset.

Parameters:
- `target` (required) — log source URI (e.g. "file://logs/validator-1/2026-03-29.jsonl")
- `text` — text substring to search for (e.g. "AppHash mismatch")
- `field` — JSON field name for field match (use with `value`, e.g. field="height", value="352922")
- `value` — value to match when field is set
- `level` — minimum log level: "debug", "info", "warn", "error"
- `module` — include only lines from this module (e.g. "consensus")
- `exclude_module` — exclude lines from this module (e.g. "p2p"). Mutually exclusive with `module`.
- `deduplicate` — when true, groups identical messages and returns counts with templates instead of individual lines (default false)
- `time_from` — start time (RFC3339, e.g. "2026-03-29T14:00:00Z")
- `time_to` — end time (RFC3339)
- `limit` — max results (default 50, max 200)

## logs_summary

Get aggregate statistics for a log file without scanning every line. Returns: total_bytes, time range (time_min, time_max), height range (height_min, height_max — best-effort from sampled lines), log level distribution (blocks_with_level counts), block_count in the index, validator_identity (extracted from privValidator/validator_address field if present), and a sample of JSON field names found. Use as the first call on a log file to understand what's in it, what heights and times it covers, and what fields are available for searching.

Required: `target` — log source URI.

## logs_navigate

Read N log lines starting from a specific time or byte offset. Stateless — no session to manage. Returns entries (line, offset, timestamp, level) with a next_offset for pagination. If the requested time is outside the file's time range, a warning field is included explaining the mismatch (e.g. "requested time is after file's last entry at ..."). Use after logs_search to explore around a match, or to page through logs sequentially. Exactly one of `time` or `offset` must be provided.

Parameters:
- `target` (required) — log source URI
- `time` — seek to this time (RFC3339, e.g. "2026-03-29T14:17:00Z")
- `offset` — seek to this byte offset; use next_offset from a previous call for pagination
- `count` — number of lines to read (default 20, max 100)

## node_data_open

Open a gno.land node's data directory and return an overview. Use as the first call when investigating offline/crashed nodes. Returns two categories of data:

**Blockstore:** chain_id, block_store_height, blockstore_app_hash (app hash from the tip block), wal_height (highest height in WAL — if > block_store_height, node was mid-consensus when it stopped).

**State.db (consensus state):** latest_height, latest_block_time, num_validators, validator addresses, state_app_hash, last_block_id (BlockID the state believes is the last committed block), last_results_hash, last_block_total_tx.

**Consistency check:** block_id_match compares state.LastBlockID against the actual BlockID of the block at that height in the blockstore. `false` means state corruption or incomplete rollback (e.g. gnobr patching only AppHash without LastBlockID). When false, blockstore_block_id shows the real BlockID for comparison.

Does NOT read block contents or transactions — use node_data_block for that.

Required: `target` — path to gnoland data directory (e.g. "/path/to/gnoland-data").

## node_data_block

Read a block from the node's local block database. Returns block header, block identity, and decoded transaction summaries. Works on offline/crashed nodes.

**Header:** height, time, chain_id, num_txs, total_txs (cumulative), proposer, app_hash.

**Block identity:** block_id (this block's BlockID: hash + parts header), last_block_id (previous block's BlockID), last_results_hash, data_hash, validators_hash, consensus_hash. Use block_id to verify against state.LastBlockID from node_data_open.

**Transactions:** each tx includes hash, message type (e.g. "/vm.m_addpkg"), sender, gas_wanted, gas_used, success/failure, error. For full tx payload (message args, package files), use node_data_tx. Does NOT query the WAL — use node_data_wal for consensus activity.

Parameters:
- `target` (required) — path to gnoland data directory
- `height` (required) — block height (0 = latest)

## node_data_tx

Decode a transaction's full payload from the local block database. Returns hash, height, index, message type, sender, gas_wanted, gas_used, success, error, fee, memo, and decoded messages array. Each message includes its type and typed details — for /vm.m_addpkg: creator, pkg_path, file names (not content); for /vm.m_call: caller, pkg_path, func, args; for /bank.MsgSend: from, to, amount. Use after node_data_block identifies a suspicious transaction. File source content is NOT included to avoid blowing context — use node_data_state to retrieve specific package source if needed.

Parameters:
- `target` (required) — path to gnoland data directory
- `hash` — transaction hash (hex, e.g. "27203B0D..."). Scans up to 10,000 blocks backward — slower than height+index.
- `height` — block height containing the tx (use with index)
- `index` — transaction index within the block (0-based, default 0)

Provide exactly one of `hash` or `height`.

## node_data_wal

Read consensus Write-Ahead Log for a specific height. Default mode is `summary` — returns a per-round digest with: round number, proposed block hash, vote tallies (prevote and precommit counts by block hash, nil count, total), and outcome (commit/timeout/wal_end). This answers "what happened in consensus?" in one call. Use `mode=raw` with filters to see individual decoded messages when you need exact vote details or timestamps.

Parameters:
- `target` (required) — path to gnoland data directory
- `height` (required) — block height to examine
- `mode` — "summary" (default) or "raw"
- `round` — filter to specific round number (raw mode, e.g. 0)
- `type` — filter by message type: "proposal", "prevote", "precommit", "timeout" (raw mode)
- `limit` — max messages in raw mode (default 50, max 200)

Raw mode requires at least one of `round` or `type` to prevent unbounded output.

## node_data_state

Query on-chain state from the IAVL tree at a specific historical height. Can look up realm/package metadata (path, file names, file sizes) or account balances (coins, sequence number). Requires the node's DB to have IAVL versions retained at the requested height — pruned heights return an error. Does NOT execute realm functions — returns stored state only.

Parameters:
- `target` (required) — path to gnoland data directory
- `height` — state version (block height; 0 = latest)
- `path` — package/realm path (e.g. "gno.land/r/demo/boards")
- `account` — account address (e.g. "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5")

Provide exactly one of `path` or `account`.

## node_compare

Compare block results across 2-5 nodes at a specific height to detect divergence. Returns a structured diff highlighting mismatches in appHash, transaction results (gas_used, success, error), and validator set. Matching fields show a single value; differing fields expand to per-node values. Use when investigating appHash divergence or consensus disagreement — the typical chain halt scenario. Does NOT compare WAL or state trees — only block-level data.

Parameters:
- `targets` (required) — array of paths to gnoland data directories (2-5)
- `height` (required) — block height to compare

## realm_eval

Evaluate a Gno expression on a running node via RPC. Use to check current realm state, verify function return values, or see a realm's display output. Returns the evaluated result as a string at the current block height. Expression `Render("")` shows the realm's main page; `Render("board/1")` shows a sub-path. Does NOT modify state — this is a read-only query. Requires an RPC target — use node_data_state to query stored state offline.

Parameters:
- `target` (required) — RPC endpoint URL (e.g. "http://node:26657")
- `pkg_path` (required) — package path (e.g. "gno.land/r/demo/boards")
- `expression` (required) — Gno expression to evaluate (e.g. "Render(\"\")" or "Counter()")

## realm_inspect

Get package overview from a running node: public function signatures, documentation, and file listing. Use to understand what a realm does before calling realm_eval. Returns functions as a formatted string, doc as text, and file names. Does NOT return file content — use realm_source to fetch specific files. Requires an RPC target — use node_data_state --path to get package metadata offline.

Parameters:
- `target` (required) — RPC endpoint URL (e.g. "http://node:26657")
- `pkg_path` (required) — package path (e.g. "gno.land/r/demo/boards")

## realm_source

Fetch a specific source file from a package on a running node. Returns the file content as a string. Source files can be large — only fetch files you actually need. Use realm_inspect first to see the file list. Does NOT return all files at once — fetch one at a time. Requires an RPC target — use node_data_state --path for offline package queries.

Parameters:
- `target` (required) — RPC endpoint URL (e.g. "http://node:26657")
- `pkg_path` (required) — package path (e.g. "gno.land/r/demo/boards")
- `file` (required) — file name to fetch (e.g. "boards.gno")

## account_info

Get account details from a running node via RPC: coin balances, sequence number, account number, and public key type. Use to check a transaction sender's identity, verify they had sufficient funds, or check account state. Returns exists=false for addresses with no on-chain activity. Requires an RPC target — use node_data_state --account for offline account queries.

Parameters:
- `target` (required) — RPC endpoint URL (e.g. "http://node:26657")
- `address` (required) — account address (e.g. "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5")

## genesis_info

Query genesis data with local caching. Genesis is immutable but can be very large (185MB+ on gnoland1, mostly balance entries). Downloads and caches the genesis file on first access at .debug/chains/<chain_id>/genesis.json — subsequent calls read from cache. Summary mode returns chain metadata, validators, and consensus params. Balance mode scans the cached file for a specific address. Works offline if genesis was previously cached. First download may take 30+ seconds for large genesis files.

Parameters:
- `target` (required) — RPC endpoint URL (e.g. "http://node:26657")
- `mode` — "summary" (default) or "balance"
- `address` — account address for balance lookup (required when mode=balance, e.g. "g1jg8mtutu9khhfwc4nxmuhcpftf0pajdhfvsqf5")

## chain_query

Raw RPC query escape hatch for operations not covered by other tools. Supports: tx (lookup transaction by hash), genesis (get genesis document), abci_query (raw ABCI state query), blockchain (list block headers in a range), dump_consensus_state (raw consensus dump with peer states). Use when you need data that no other tool provides. For common operations, prefer node_overview (node status), block_inspect (block details), peer_consensus (peer consensus state), or node_data_* tools (offline analysis).

Parameters:
- `target` (required) — RPC endpoint URL
- `method` (required) — one of: "tx", "genesis", "abci_query", "blockchain", "dump_consensus_state"
- `params` — method-specific parameters (tx: {hash}; genesis: {}; abci_query: {path, data, height}; blockchain: {min_height, max_height}; dump_consensus_state: {})

## peer_consensus

Tool to show the consensus state of each connected peer. Use when investigating consensus progress across the network, especially when some validators aren't directly reachable via RPC. Returns each peer's height, round, step, and which validator votes they've seen (expanded bitarrays with voter indices). Also includes the local node's own consensus state and a summary of peer alignment. Does NOT show individual vote details (which block hash was voted for) — use node_data_wal for that.

Required: `target` — RPC endpoint URL (e.g. "http://node:26657").

## node_doctor

Run comprehensive health checks against a gno.land node. Accepts an RPC endpoint or data directory path. Detects what sources are available and runs all applicable checks in one call. Returns structured findings with severity levels (critical, warning, info), a context summary (height, block age, peers, consensus state, validators, WAL status), and correlation-based suggestions. Use as the first call when starting an investigation — replaces the manual orient phase of calling node_overview, block_inspect, and genesis_info separately.

Parameters:
- `target` (required) — RPC endpoint URL (e.g. "http://node:26657") or path to gnoland data directory
