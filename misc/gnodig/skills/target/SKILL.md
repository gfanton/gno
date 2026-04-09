---
name: target
description: Manage chain/node target configurations for gnodig investigations. Use /target to add, list, update, show, or remove node endpoints and data paths.
argument-hint: <add|list|update|show|remove> [chain] [moniker]
---

# Target Management

Manage node identity registry stored in `.debug/nodes/`. These are lightweight identity files — bulk data (logs, DB snapshots) belongs in session artifacts, not here.

## Usage

```
/target add <chain> <moniker>       # starts interactive prompt for fields
/target list [chain]
/target update <chain> <moniker>    # asks which fields to change
/target show <chain> <moniker>
/target remove <chain> <moniker>
```

## Behavior

**add:** Ask the user for each field interactively. Required: chain and moniker (from arguments). Then ask: "RPC endpoint?", "Role (validator/rpc)?", "Validator address?", "Any notes?". Skip fields the user doesn't have. Create `.debug/nodes/<chain>/<moniker>.yaml`. Fail if the node already exists (use `update` instead).

**list:** Show all chains and their nodes. If `chain` is specified, show only that chain's nodes. Display moniker, role, RPC endpoint for each node.

**update:** Show the current config, then ask: "Which fields do you want to update?" Accept answers conversationally — the user might say "change the RPC to http://..." or "add a note about the restart".

**show:** Display the full node yaml contents for a specific node.

**remove:** Delete the node file. Ask for confirmation before deleting.

## Node Config Format

```yaml
moniker: val01
chain_id: gno-mainnet
role: validator
address: g1...
rpc: http://val01:26657
probe: val01.internal:9090       # Optional: probe sidecar address
probe_pubkey: ed25519:abc123...  # Optional: probe server's expected pubkey
notes: ""
```

Identity only — no data_dir or logs paths. Those belong in session artifacts.

`probe` and `probe_pubkey` are optional. When `probe` is set, offline tools (`node_data_*`, `logs_*`) route through the probe client instead of direct filesystem access. When only `rpc` is set, behavior is unchanged. `probe_pubkey` is reserved for future server identity verification.

## Arguments

Parse `$ARGUMENTS` to determine the subcommand and chain/moniker. If no arguments are given, default to `list`. For `add` and `update`, use interactive prompts for optional fields — do NOT parse CLI flags.

## Directory Structure

```
.debug/
  nodes/
    <chain>/
      <moniker>.yaml
```

Create `.debug/nodes/` and subdirectories as needed. This directory is git-ignored.
