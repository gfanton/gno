# Postmortem: {title}

**Date:** {date}
**Chain:** {chain-id}
**Severity:** critical | high | medium | low
**Session:** .debug/sessions/{slug}/

## Incident Summary

{1-2 paragraph summary: what happened, when, impact}

## Timeline

| Time | Event |
|------|-------|
| | First symptom observed |
| | Investigation started |
| | Root cause identified |
| | Resolution / mitigation |

## Root Cause

{Confirmed root cause or best hypothesis with confidence level}

## Evidence Chain

{How one finding led to another — the reasoning path}

1. Started with: {initial evidence}
2. This led to: {first finding}
3. Which suggested: {hypothesis}
4. Confirmed by: {decisive evidence}

## What Worked

- {Tool/approach that helped}

## What Didn't Work (Dead Ends)

- {Approach that was tried but didn't help, and why}

## Red Herrings

- {Things that looked suspicious but turned out to be irrelevant}

## Recommendations

- {Actions to prevent recurrence or improve detection}

## Playbook Candidate

{If this investigation revealed a new debugging pattern, describe it here for distillation into a shared playbook}

### Symptom: {what the operator sees}
**Likely cause:**
**Check first:**
**Check second:**
**Red herring:**
**Code paths:**
**MCP tools:**
**Reference:**
