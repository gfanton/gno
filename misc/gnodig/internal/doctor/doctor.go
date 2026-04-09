package doctor

import (
	"context"
	"errors"
	"fmt"

	"github.com/gnolang/gno/misc/gnodig/internal/chainrpc"
	"github.com/gnolang/gno/misc/gnodig/internal/nodedata"
)

var defaultChecks = []Check{
	{ID: "chain_halted", Run: checkChainHalted},
	{ID: "consensus_stuck", Run: checkConsensusStuck},
	{ID: "low_peers", Run: checkLowPeers},
	{ID: "mempool_backlog", Run: checkMempoolBacklog},
	{ID: "wal_ahead", Run: checkWALAhead},
	{ID: "vote_split", Run: checkVoteSplit},
	{ID: "proposal_rejected", Run: checkProposalRejected},
	{ID: "state_mismatch", Run: checkStateMismatch},
}

var defaultCorrelations = []Correlation{
	{ID: "apphash_divergence", Run: correlateAppHashDivergence},
	{ID: "validators_offline", Run: correlateValidatorsOffline},
	{ID: "crash_recovery", Run: correlateCrashRecovery},
}

func runChecks(ctx *Context) *Report {
	report := &Report{
		Target:  ctx.target,
		Sources: make(map[string]bool),
	}

	// Detect available sources.
	if _, err := ctx.status.Get(); err == nil {
		report.Sources["rpc"] = true
	}
	if _, err := ctx.dataOverview.Get(); err == nil {
		report.Sources["data_dir"] = true
	}

	// Run all checks, silently skipping those whose provider is unavailable.
	for _, check := range defaultChecks {
		findings, err := check.Run(ctx)
		if err != nil {
			if errors.Is(err, ErrProviderNotAvailable) {
				continue
			}
			report.Errors = append(report.Errors, CheckError{
				CheckID: check.ID,
				Error:   err.Error(),
			})
			continue
		}
		report.Findings = append(report.Findings, findings...)
	}

	// Run correlations over collected findings.
	for _, corr := range defaultCorrelations {
		extra := corr.Run(report.Findings, ctx)
		report.Findings = append(report.Findings, extra...)
	}

	// Healthy if no Critical or Warning findings.
	report.Healthy = true
	for _, f := range report.Findings {
		if f.Severity == Critical || f.Severity == Warning {
			report.Healthy = false
			break
		}
	}

	return report
}

// RunDoctor is the public entry point. It wires providers from the given
// RPC client and/or data directory, then runs all checks and correlations.
func RunDoctor(ctx context.Context, target string, rpcClient *chainrpc.Client, dataDir *nodedata.DataDir) *Report {
	dctx := &Context{
		target: target,
		ttype:  DetectTargetType(target),
	}

	if rpcClient != nil {
		dctx.status = newProvider(func() (*chainrpc.Overview, error) {
			ov := chainrpc.FetchOverview(ctx, rpcClient)
			if ov.StatusError != "" {
				return ov, fmt.Errorf("rpc status: %s", ov.StatusError)
			}
			return ov, nil
		})
	}

	if dataDir != nil {
		dctx.dataOverview = newProvider(func() (*nodedata.Overview, error) {
			return dataDir.Overview()
		})
		dctx.walSummary = newProvider(func() (*nodedata.WALSummary, error) {
			ov, err := dctx.dataOverview.Get()
			if err != nil {
				return nil, err
			}
			return dataDir.WALSummary(ov.WALHeight)
		})
	}

	return runChecks(dctx)
}
