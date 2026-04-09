package doctor

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCorrelateAppHashDivergence_Matches(t *testing.T) {
	findings := []Finding{
		{ID: "chain_halted", Severity: Critical},
		{ID: "vote_split", Severity: Critical},
	}
	result := correlateAppHashDivergence(findings, nil)
	assert.Len(t, result, 1)
	assert.Equal(t, "apphash_divergence", result[0].ID)
	assert.Equal(t, Critical, result[0].Severity)
	assert.Contains(t, result[0].Detail, "confidence: high")
}

func TestCorrelateAppHashDivergence_MatchesWithProposalRejected(t *testing.T) {
	// The block-vs-nil pattern (proposal_rejected) should also trigger this correlation
	findings := []Finding{
		{ID: "chain_halted", Severity: Critical},
		{ID: "proposal_rejected", Severity: Warning},
	}
	result := correlateAppHashDivergence(findings, nil)
	assert.Len(t, result, 1)
	assert.Equal(t, "apphash_divergence", result[0].ID)
}

func TestCorrelateAppHashDivergence_NoMatch(t *testing.T) {
	findings := []Finding{
		{ID: "chain_halted", Severity: Critical},
	}
	result := correlateAppHashDivergence(findings, nil)
	assert.Empty(t, result)
}

func TestCorrelateValidatorsOffline_Matches(t *testing.T) {
	findings := []Finding{
		{ID: "chain_halted", Severity: Critical},
		{ID: "validator_missing", Severity: Warning},
	}
	result := correlateValidatorsOffline(findings, nil)
	assert.Len(t, result, 1)
	assert.Equal(t, "validators_offline", result[0].ID)
	assert.Equal(t, Critical, result[0].Severity)
	assert.Contains(t, result[0].Detail, "confidence: high")
}

func TestCorrelateValidatorsOffline_NoMatch_HasVoteSplit(t *testing.T) {
	findings := []Finding{
		{ID: "chain_halted", Severity: Critical},
		{ID: "validator_missing", Severity: Warning},
		{ID: "vote_split", Severity: Critical},
	}
	result := correlateValidatorsOffline(findings, nil)
	assert.Empty(t, result)
}

func TestCorrelateCrashRecovery_Matches(t *testing.T) {
	findings := []Finding{
		{ID: "wal_ahead", Severity: Info},
		{ID: "state_mismatch", Severity: Critical},
	}
	result := correlateCrashRecovery(findings, nil)
	assert.Len(t, result, 1)
	assert.Equal(t, "crash_recovery", result[0].ID)
	assert.Equal(t, Warning, result[0].Severity)
	assert.Contains(t, result[0].Detail, "confidence: medium")
}

func TestCorrelateCrashRecovery_NoMatch(t *testing.T) {
	findings := []Finding{
		{ID: "wal_ahead", Severity: Info},
	}
	result := correlateCrashRecovery(findings, nil)
	assert.Empty(t, result)
}
