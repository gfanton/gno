# Artifact Management

Raw investigation data (downloaded DBs, logs, WAL copies) lives under `.debug/sessions/<slug>/artifacts/` organized by node moniker.

## Directory Structure

```
artifacts/
├── validator-1/
│   ├── db/          (blockstore, state, gnolang PebbleDB files)
│   ├── wal/         (consensus WAL)
│   └── logs.jsonl   (node logs)
├── validator-2/
│   └── db/
└── notes.md         (index: what's here, when captured, how)
```

## When to Ask

The agent asks about artifacts at three moments:

1. **When evidence arrives** (during Orient or mid-course):
   > "Want me to download this into session artifacts, or reference the original path?"

2. **When referencing remote data** (rsync, scp, etc.):
   > "Store in artifacts/<moniker>/ or use /tmp?"

3. **At close:**
   > "Session artifacts have ~X of data. Keep or clean up?"

## notes.md

Every artifacts directory should have a `notes.md` that indexes what's available:

```markdown
# Artifacts Index

## validator-1
- **db/** — blockstore (579MB), state (20MB), gnolang (34MB). Downloaded via rsync.
- **wal/** — consensus WAL. 102 segment files. Downloaded same session.
- **logs.jsonl** — 24h docker logs, 897MB, 1.8M lines. Captured via `docker logs --since 24h`.

## validator-2
- **db/** — blockstore (576MB), state (14MB), gnolang (26MB). Downloaded from backup.
```

## PebbleDB Lock Files

When downloading PebbleDB databases from remote nodes, rsync copies `LOCK` files. Remove them before opening with gnodig:

```bash
find artifacts/<moniker>/db -name LOCK -delete
```

If gnodig reports "database locked by another process" on data that isn't locked, the database may be corrupt (truncated SST files from rsyncing a live DB). Check the actual error with a direct PebbleDB open — gnodig's `isLockError` heuristic can misclassify corruption as lock errors.
