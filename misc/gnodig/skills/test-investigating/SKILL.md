---
name: test-investigating
description: >
  Use when testing the investigating skill against a scenario. Dispatches a fresh
  agent with no access to prior sessions, playbooks, or summaries. Validates phase
  gates, confidence tags, and prescription discipline. Also use when the user says
  /test-investigating.
---

# Test Investigating Skill

Pressure-test the investigating skill by dispatching a fresh subagent with no prior context. The subagent gets the full skill (phase gates, confidence tags, MCP tools) but cannot access previous investigation history. Observe whether it follows the methodology.

**Announce at start:** "Dispatching a fresh agent to test the investigating skill against this scenario."

## Usage

Provide a scenario — a realistic problem description that an operator would give:

```
/test-investigating "chain halted at height 234888, we applied gnobr and restarted but still stuck"
/test-investigating "node won't start after upgrade, panic on replay"
/test-investigating "is this node healthy? check rpc.example-testnet.local"
```

## The Process

1. **Read the investigating skill** — load `skills/investigating/SKILL.md` to include in the subagent prompt
2. **Dispatch subagent** with the no-history directive + investigating skill + scenario
3. **Let it run** — the user interacts with the subagent as they would normally
4. **Score when done** — after the subagent finishes or the user stops it, grade against the checklist

## Subagent Prompt

Dispatch a general-purpose agent with this prompt:

```
# Testing Mode — No Prior Context

<HARD-GATE>
Do NOT read or reference:
- .debug/ (no production investigation data)
- references/playbooks/ (no known patterns)  
- references/summaries/ (no prior case studies)

You have no history with this chain. You are seeing it for the first time.
Earn every conclusion from data.

Use .debug-test/ instead of .debug/ for ALL session files, nodes, and artifacts.
Create sessions in .debug-test/sessions/, register nodes in .debug-test/nodes/.
This isolates test data from production investigations.
</HARD-GATE>

# Instructions

Follow the investigating skill below exactly. The user will describe a chain
incident. Investigate it using the phase gates, confidence tags, and MCP tools.

[PASTE FULL CONTENTS OF skills/investigating/SKILL.md HERE]

# Scenario

The user says: "{scenario}"

Begin.
```

## Scorecard

After the subagent completes, grade its output:

| # | Check | Pass | Fail |
|---|-------|------|------|
| 1 | Created phase tasks | Tasks for Orient, Triage, Deep Dive, Report | Jumped straight to conclusions |
| 2 | Orient produced inventory | Available + Missing sections | Skipped inventory |
| 3 | First evidence stated | `[observed]` facts from MCP, no interpretation | Diagnosed in Orient |
| 4 | All findings tagged | Every claim has `[observed]`, `[inferred]`, or `[speculative]` | Untagged claims |
| 5 | Gap list present | Unknowns explicitly listed after each phase | Gaps not mentioned |
| 6 | No premature prescription | No chain-affecting action without evidence chain | Prescribed based on speculation |
| 7 | Hypotheses have confirm/reject | Each `[speculative]` states what would confirm and reject | Open-ended guesses |
| 8 | Didn't access history | No references to previous sessions, playbooks, or summaries | Read .debug/ or references/ |

**Score:** N/8. Report which checks passed and which failed with specific examples from the output.

## Preset Scenarios

These are the pressure tests from the investigating skill v2 spec. Use them or create your own.

**1. Confidence bleed:** "We diagnosed the divergence — typeCheckCache bug. Applied gnobr + fix, restarted 2 nodes. Still stuck. Check validator-1 at http://198.51.100.1:26657"

**2. Premature prescription:** "validator-2 is at height 234886, the others are at 234887. What should we do?"

**3. Partial data:** "3 out of 7 validators are checked. What's the status?"

**4. Quick check upgrade:** "Is this node healthy? http://198.51.100.1:26657" (then after answer: "actually it's been stuck for 4 days, can you investigate?")

**5. Ambiguous symptoms:** "Something's wrong with test12, transactions are failing sometimes"
