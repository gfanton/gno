# Phase Gates Reference

Detailed phase descriptions, gate conditions, confidence rules, and prescription logic for the investigating skill. SKILL.md has brief summaries — this file has the full details.

## Table of Contents

- [Confidence Tags](#confidence-tags)
- [Phase 1: Orient](#phase-1-orient)
- [Phase 2: Triage](#phase-2-triage)
- [Phase 3: Deep Dive](#phase-3-deep-dive)
- [Phase 4: Report](#phase-4-report)
- [Phase 5: Close](#phase-5-close)
- [Investigation Continuity: Passes](#investigation-continuity-passes)
- [Living Inventory](#living-inventory)
- [Outputs](#outputs)
- [Task Template](#task-template)

---

## Confidence Tags

Every finding must be tagged. No exceptions.

| Tag | Meaning | Rule |
|-----|---------|------|
| `[observed]` | Directly from MCP tool output, logs, or user statement | Cite the source |
| `[inferred]` | Logical deduction from multiple observations | Cite which observations |
| `[speculative]` | Hypothesis needing more evidence | State what would confirm AND reject |

### Examples (from test12 investigation)

**Observed:**
> validator-1 at height 234887, consensus 234888/0/4 `[observed]` (source: RPC node_overview)

**Inferred:**
> Chain needs 1 more validator to reach consensus `[inferred]` (from: 4/7 prevotes observed via logs, need 5/7 for 2/3+)

**Speculative:**
> validator-2 may be at a different height due to different gnobr params `[speculative]` (would confirm: operator confirms --drop-after value; would reject: validator-2 logs show replay stopped at 234886 for other reason)

### Hard Rule

<HARD-GATE>
Never prescribe chain-affecting actions (gnobr, restart, state patches) based on [speculative] findings. If the evidence chain for a recommendation contains any [unknown] or [speculative] step, the recommendation is BLOCKED. Ask for data instead.
</HARD-GATE>

---

## Phase 1: Orient

**Answers:** "What do I have, what don't I have, and what's the situation?"

### Steps

1. **Ask what evidence exists:**
   - RPC endpoint to a running node?
   - Data directory from a stopped/crashed node?
   - Log files from the incident?
   - Access to multiple nodes for comparison?
   - What chain? What height (if known)?

2. **Cross-session check:** Scan `.debug/sessions/` for previous sessions on the same chain. Read their headers (Status, Chain, Incident Description). If related: "I see a previous session on this chain. Should I review it for context?"

3. **Make first MCP calls:**
   - **Recommended:** `node_doctor` for comprehensive health check
   - Live node → `node_overview`, `account_info`, `realm_eval`, `genesis_info` as needed
   - Offline node → `node_data_open` for height, validators, WAL status, state consistency
   - Logs available → `logs_summary` for time range, levels, fields

4. **Write the inventory:**
   - **Available:** what evidence exists and what it showed
   - **Missing:** what evidence is not available

5. **First evidence statement:** Brief factual summary of the big picture. Strictly `[observed]`, no interpretation.

   Good examples:
   - "Chain is halted at height 234888. Last block was 4 days ago. 4 peers connected."
   - "Node is healthy. Blocks producing normally, 6 peers, no consensus issues."
   - "Node is at height 234886, peers are at 234887. Consensus stuck at step 8."
   - "RPC not responding. Data directory available for offline analysis."

   Bad examples:
   - "Chain halted because of typeCheckCache divergence" — this is diagnosis, not evidence
   - "Node was gnobr'd to the wrong height" — this is speculation

6. **Offer optionals:** Session file? Node registration? Artifact storage?

### Gate

Inventory is written (Available + Missing). First evidence stated. User has confirmed what's available.

**Orient produces inventory and first evidence, not a diagnosis.**

---

## Phase 2: Triage

**Answers:** "What kind of problem is this?"

### Two Paths

**Path A — Classification supported by data:**
If Orient's MCP calls make it clear (e.g., `node_doctor` returns `chain_halted` + `appHash_divergence`), classify directly. The classification must be tagged `[observed]` or `[inferred]` with cited evidence.

**Path B — Symptoms ambiguous:**
Dispatch `gno-investigator` agent with the Orient inventory. The investigator returns: classification, recommended experts, evidence gaps, initial hypothesis.

### Outputs
- Classification with confidence tag and cited evidence
- Recommended expert(s) to dispatch
- Updated gap list
- Initial hypotheses — explicitly `[speculative]`, with confirm/reject criteria

### Gate

Classification is `[observed]` or `[inferred]` with cited evidence. If the best classification is `[speculative]`, acknowledge this explicitly and proceed with stated uncertainty.

**Triage doesn't need to be 100% certain.** "This looks like a consensus halt, possibly appHash divergence, but I can't confirm without WAL data `[speculative — need WAL]`" is perfectly valid. The discipline is honesty about confidence, not certainty.

---

## Phase 3: Deep Dive

**Answers:** "What specifically happened and why?"

### How It Works

1. **Dispatch domain expert(s)** based on Triage classification. Each expert receives:
   - Current inventory (Available/Missing)
   - Triage classification with confidence tag
   - Relevant playbook(s) from `references/playbooks/`
   - Session context from previous passes (if any)
   - Instruction: tag every finding, verify hypotheses against evidence

2. **Iterative evidence gathering.** After each MCP call or expert return:
   - Write finding with confidence tag to session.md
   - Update gap list
   - Update inventory if new evidence appeared
   - Check: does this confirm, reject, or modify a hypothesis?

3. **Hypothesis tracking.** Maintain a running list:
   ```
   - H1: typeCheckCache divergence [speculative → inferred → confirmed]
     Evidence: node_compare showed 40% gas diff for same tx
   - H2: validator misconfiguration [speculative → rejected]
     Evidence: all validators running same binary version
   ```

### Discipline
- Every finding gets a tag. No exceptions.
- Gap list updated after every tool call. Not batched.
- When forming a hypothesis, state what evidence would **confirm** AND what would **reject** it (prevents confirmation bias).

### No Fixed End

Deep Dive loops until:
- Root cause is confirmed (`[observed]` or `[inferred]` with strong evidence)
- Investigation hits a wall (not enough data, need operator action)
- User decides to pause

---

## Phase 4: Report

**Answers:** "Here's what I know, what I don't know, and what I think."

### Report Structure

```
## Observations (from data)
- [observed] ... (source: ...)

## Inferences (from multiple observations)
- [inferred] ... (from: ...)

## Unknowns (no data)
- ...

## Hypotheses (need more evidence)
- [speculative] ... (would confirm: ... / would reject: ...)
```

### Prescription Gate

Before recommending any chain-affecting action, produce an evidence chain:

```
## Recommended Action: [action]
## Evidence Chain:
1. [observed] ... (source: ...)
2. [inferred] ... (from: ...)
3. [unknown] ...  ← BLOCKS RECOMMENDATION

## Verdict: CANNOT RECOMMEND — step 3 is unknown.
## Instead: Ask [who] for [what data].
```

If any step is `[unknown]`, the recommendation is blocked. Ask for data instead.

**User override:** The user can say "do it anyway." Document: "Proceeding despite unknown at step N, per user instruction."

---

## Phase 5: Close

**Triggers:** Root cause resolved, user pauses, or no more data available.

### Offered Optionals

Present in one message, user picks:

| Output | Where | Public? | Purpose |
|--------|-------|---------|---------|
| Report | `.debug/sessions/<slug>/report.md` | No | Full internal record |
| Investigation summary | `references/summaries/` | Yes (repo) | Sanitized case study |
| Playbook entry | `references/playbooks/` | Yes (repo) | Short pattern match |
| Doctor improvement | Code via `/add-symptom` | Yes (repo) | New health check |
| Artifact cleanup | Delete artifacts/ | N/A | Reclaim space |

**Offer format:**
> "Investigation wrapping up. I can:
> 1. Write a formal report? (internal, full details)
> 2. Write an investigation summary? (publishable case study)
> 3. Propose a playbook entry? (short pattern for future investigations)
> 4. Add a doctor health check? (node_doctor doesn't detect [X] yet)
> 5. Clean up artifacts? (~X in session dir)
>
> Which of these? (all/some/none)"

### Pause vs Close

- **Pause:** Session stays open. Write resumption context. Keep artifacts. Status → `paused`.
- **Close:** Full close sequence. All optionals offered. Status → `closed`.

---

## Investigation Continuity: Passes

An investigation has one or more **passes**. Each pass goes through phases independently but shares the session's accumulated findings.

### When Context Shifts

Context shifts when: user reports "fix applied but still stuck", new evidence arrives, recovery phase begins, operator takes action and situation changes.

1. Announce: "Situation changed. Starting a new pass."
2. Create new phase tasks.
3. In Orient, **read previous pass findings** — don't lose context.
4. Gap list carries forward — unknowns still unknown stay on the list.
5. Findings from previous passes are preserved with their pass number.

### Key Rule

**A context shift is a new pass, not a new investigation.** Previous findings are carried, not discarded. But confidence levels reset — earned confidence from Pass 1 does not transfer to Pass 2.

---

## Living Inventory

The inventory from Orient is not frozen. New evidence can appear at any point:
- User provides new data mid-investigation
- MCP call reveals something unexpected
- Expert dispatch identifies a new source

**Rule:** When new evidence appears, update the inventory immediately. Append to Available/Missing and tag the source. No need to re-run Orient.

---

## Outputs

### Mandatory (always produced)

| Output | When | Content |
|--------|------|---------|
| Findings with tags | After every observation | `[observed]`, `[inferred]`, `[speculative]` on every claim |
| Gap list | After every phase and tool call | What's missing, what questions remain |
| Evidence chain | Before any prescription | Linked chain, blocked if `[unknown]` steps exist |

### Optional (always offered at the right moment)

| Output | When offered | Content |
|--------|-------------|---------|
| Session file | Start of full investigation | Persistent recovery point |
| Node registration | When new nodes encountered | Save to `.debug/nodes/` |
| Report | At close | Full internal record (sensitive) |
| Investigation summary | At close | Sanitized case study (publishable) |
| Playbook entry | At close | Short pattern match |
| Doctor improvement | When reproducible symptom found | New health check via `/add-symptom` |
| Artifact cleanup | At close | Reclaim disk space |

---

## Task Template

When starting a full investigation pass, create tasks:

```
- [ ] Orient — inventory + first evidence + gaps
      Done when: Available/Missing written, first evidence stated, user confirmed
- [ ] Triage — classify incident
      Done when: classification tagged [observed] or [inferred] with cited evidence
- [ ] Deep Dive — experts + evidence gathering
      Done when: hypotheses tracked, every finding tagged, gap list current
- [ ] Report — present findings
      Done when: user has seen Observations/Inferences/Unknowns/Hypotheses
- [ ] Prescription gate — evidence chain (if recommending actions)
      Done when: evidence chain complete with no [unknown] steps, or user informed of unknowns
- [ ] Close or Pause
      Done when: optionals offered, session status updated
```

When context shifts, create a new set of tasks for the new pass.
