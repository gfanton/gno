# Investigation Summary: {Title} ({chain}, {date})

## Symptom

{What was observed — no sensitive details. Describe the user-facing impact.}

## Classification

{Incident type chain, e.g.: consensus halt → appHash divergence → non-deterministic gas}

## Diagnostic Path

{Numbered steps showing which tools were used, in what order, and what they revealed.
This is the most valuable part — it teaches others how to diagnose similar issues.}

1. `node_doctor` → {what it found}
2. `node_data_open` on two nodes → {what it showed}
3. `node_compare` at height {N} → {the key diff}
4. `node_data_wal` → {vote pattern}
5. `logs_search` → {what confirmed the diagnosis}

## Root Cause

{Technical explanation of what went wrong and why.}

## Key Indicators

{Pattern-matchable signals for future investigations. What should trigger
someone to suspect this same issue?}

- {Signal 1}
- {Signal 2}
- {Signal 3}

## Recovery

{What was done to resolve. Include blockers encountered and how they were overcome.}

## Lessons

- {What worked well in the investigation}
- {What didn't work or was misleading}
- {What to do differently next time}
