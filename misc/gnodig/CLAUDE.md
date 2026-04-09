# gnodig — Chain Investigation Toolkit for gno.land

## What This Is

gnodig is an MCP server + Claude Code plugin for investigating gno.land chain incidents.
Two modes: live node (RPC) and offline node (data directory analysis).

## Build & Run

```bash
go run ./cmd/gnodig serve        # Start MCP server (stdio)
go test ./...                    # Run all tests
```

## Project Layout

| Directory | Purpose |
|-----------|---------|
| `cmd/gnodig/` | CLI entry point |
| `internal/mcp/` | MCP server, tool registration, descriptions |
| `internal/chainrpc/` | RPC client for live node queries |
| `internal/logengine/` | Log indexing, search, dedup, cursor-based navigation |
| `internal/nodedata/` | Offline data dir analysis (blocks, WAL, state, tx) |
| `internal/driver/` | Pluggable log source drivers |
| `skills/` | Claude Code skills (investigating, /target, /session) |
| `agents/` | Expert agents (gno-investigator, tm2-expert, gnovm-expert, etc.) |
| `.docs/` | Authoring guides — read before writing MCP tools or skills |

## Documentation Standards

**Before writing or modifying MCP tools**: read `.docs/write_mcp.md`. Key rules:
- Tool descriptions: 3-4 sentences minimum, "Tool to [what]. Use when [situation]."
- Parameter descriptions: name, format, constraints, examples, conditional rules.
- Document what the tool does NOT return.
- Update `internal/mcp/mcp.md` with any tool description changes.

**Before writing or modifying skills**: read `.docs/write_skill.md` AND `.docs/write_mcp.md`. Key rules:
- Description = triggering conditions, never workflow summary.
- SKILL.md body under 500 lines; heavy content in `references/`.
- Anti-pattern section, checklist, flowchart, red-flag table.
- Test with fresh agent instance (Claude A/B pattern).
- **Skills live in `skills/`** — `.claude/skills` is a symlink to `skills/`. Always edit files under `skills/` directly.

**Before writing or modifying agents**: agents are `.md` files with YAML frontmatter (name, description, model, tools). Body is the agent's system prompt. Include domain-specific reasoning heuristics, not just tool instructions.
- **Agents live in `agents/`** — `.claude/agents` is a symlink to `agents/`. Always edit files under `agents/` directly.

## MCP Tool Descriptions

All tool descriptions live in `internal/mcp/mcp.md` as markdown sections. The `descriptions.go` file loads them at registration time via `desc("section_name")`. When adding or changing a tool:
1. Write the description in `mcp.md` first.
2. Register the tool in the appropriate `tools_*.go` file.
3. Run `go test ./internal/mcp/...` to verify description coverage.

## Keeping Things In Sync

When you change any part of the system, update all downstream references. This is non-negotiable.

| You changed... | Also update... |
|----------------|----------------|
| MCP tool (add/rename/modify) | `internal/mcp/mcp.md` description, `mcp.md#instructions` summary, agent playbooks that reference the tool |
| MCP tool parameters | `mcp.md` parameter docs, skill instructions if they reference params |
| Agent playbook | Skill `references/playbooks/` (shared), verify agent `.md` still references correct playbook files |
| Skill workflow | Skill `SKILL.md`, flowchart, checklist — all three must agree |
| Chain profile | `skills/investigating/references/chains/*.yaml` |
| `.docs/` authoring guides | Nothing downstream, but re-read before next edit to anything they cover |

**The `mcp.md#instructions` section is the MCP server's system prompt.** It's the first thing any agent sees. When tools are added, removed, or renamed, this section MUST reflect the current tool inventory and recommended investigation flow.

## Conventions

- Follow parent repo conventions (see root `AGENTS.md`): conventional commits, kebab-case branches.
- Go code: `gofmt`, `golangci-lint`, standard error handling.
- MCP tools: read-only by default (`readOnlyAnnotation`). Destructive tools need explicit annotation.
- Skill names: kebab-case, gerund form preferred (`investigating`, not `investigate`).
- Agent playbooks in `skills/investigating/references/playbooks/` are **shared knowledge** — version-controlled, PR'd, reviewed. Sessions in `.debug/` are personal and git-ignored.
