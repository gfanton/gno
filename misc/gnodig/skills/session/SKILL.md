---
name: session
description: Manage investigation sessions for gnodig. Use /session to list, resume, or close investigation sessions.
argument-hint: <list|resume|close> [slug]
---

# Session Management

Manage investigation sessions stored in `.debug/sessions/`.

## Usage

```
/session list                    # Show all sessions
/session resume <slug>           # Reload session context
/session close                   # Close current session with postmortem
```

## Behavior

**list:** Show all sessions in `.debug/sessions/` with their status (read from the `Status:` field in each session.md), chain, creation date, and a one-line incident summary.

**resume:** Read `.debug/sessions/<slug>/session.md` and re-hydrate the investigation context. Present a summary of:
- What the incident was about
- What evidence was collected
- What artifacts are available (list `artifacts/` subdirectory contents)
- What hypotheses were active
- What was the last finding
- What open questions remain
Also check if newer sessions exist on the same chain — they may contain relevant follow-up findings. Then ask the user how they'd like to continue.

**close:** Triggers the mandatory close sequence:

1. **Generate postmortem:** Read the session journal and synthesize a postmortem using the template at `skills/investigating/references/templates/postmortem.md`. Write to `.debug/sessions/<slug>/postmortem.md`.

2. **Propose new playbook:** Extract debugging patterns learned during this investigation. Show the proposed playbook entry to the user for review. If approved, write as a new file: `skills/investigating/references/playbooks/YYYY-MM-DD-<slug>.md`. This file is in the repo (not .debug/) so the user can commit and PR it.

3. **Update metadata:** If the investigation revealed chain/node knowledge (problematic heights, node quirks), update the relevant files:
   - `skills/investigating/references/chains/*.yaml` for shared chain knowledge
   - `.debug/nodes/<chain>/<moniker>.yaml` for personal node notes

4. **Mark session closed:** Update the `Status:` field in session.md to `closed`.

## Arguments

Parse `$ARGUMENTS` to determine the subcommand. If no arguments are given, default to `list`.

## Directory Structure

```
.debug/
  nodes/
    <chain>/
      <moniker>.yaml      # Node identity (persistent across sessions)
  sessions/
    <YYYY-MM-DD-slug>/
      session.md          # Investigation journal
      postmortem.md       # Generated on close
      artifacts/          # Session-owned data
        <moniker>/        # Per-node evidence
          gnoland-data/
          gnoland.log
```
