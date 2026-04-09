package doctor

import "slices"

func hasFind(findings []Finding, id string) bool {
	return slices.ContainsFunc(findings, func(f Finding) bool {
		return f.ID == id
	})
}

func correlateAppHashDivergence(findings []Finding, _ *Context) []Finding {
	if !hasFind(findings, "chain_halted") {
		return nil
	}
	if !hasFind(findings, "vote_split") && !hasFind(findings, "proposal_rejected") {
		return nil
	}
	return []Finding{{
		ID:       "apphash_divergence",
		Severity: Critical,
		Detail:   "Chain halted with vote disagreement — likely appHash divergence (confidence: high)",
		Source:   "correlation",
	}}
}

func correlateValidatorsOffline(findings []Finding, _ *Context) []Finding {
	if !hasFind(findings, "chain_halted") || !hasFind(findings, "validator_missing") {
		return nil
	}
	if hasFind(findings, "vote_split") {
		return nil
	}
	return []Finding{{
		ID:       "validators_offline",
		Severity: Critical,
		Detail:   "Chain halted with missing validators but no vote split — likely insufficient voting power (confidence: high)",
		Source:   "correlation",
	}}
}

func correlateCrashRecovery(findings []Finding, _ *Context) []Finding {
	if !hasFind(findings, "wal_ahead") || !hasFind(findings, "state_mismatch") {
		return nil
	}
	return []Finding{{
		ID:       "crash_recovery",
		Severity: Warning,
		Detail:   "WAL ahead of blockstore with state mismatch — node crashed mid-consensus, state may be inconsistent (confidence: medium)",
		Source:   "correlation",
	}}
}
