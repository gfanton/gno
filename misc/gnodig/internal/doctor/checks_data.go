package doctor

import "fmt"

func checkWALAhead(ctx *Context) ([]Finding, error) {
	ov, err := ctx.dataOverview.Get()
	if err != nil {
		return nil, err
	}
	if ov.WALHeight <= ov.BlockStoreHeight {
		return nil, nil
	}
	return []Finding{{
		ID:       "wal_ahead",
		Severity: Info,
		Detail:   fmt.Sprintf("WAL height %d > blockstore height %d — node stopped mid-consensus", ov.WALHeight, ov.BlockStoreHeight),
		Source:   "data_dir",
	}}, nil
}

func checkVoteSplit(ctx *Context) ([]Finding, error) {
	ws, err := ctx.walSummary.Get()
	if err != nil {
		return nil, err
	}
	// Report only the first round with a split — the earliest split
	// is the root cause; later rounds inherit the disagreement.
	for _, round := range ws.Rounds {
		if len(round.Prevotes.Block) > 1 {
			return []Finding{{
				ID:       "vote_split",
				Severity: Critical,
				Detail:   fmt.Sprintf("WAL round %d at height %d: %d distinct prevote hashes — validators disagree on block", round.Round, ws.Height, len(round.Prevotes.Block)),
				Source:   "data_dir",
			}}, nil
		}
	}
	return nil, nil
}

func checkProposalRejected(ctx *Context) ([]Finding, error) {
	ws, err := ctx.walSummary.Get()
	if err != nil {
		return nil, err
	}
	for _, round := range ws.Rounds {
		// Majority voted nil while a minority voted for a block = proposal rejection.
		// All nil with no block votes = proposer timeout, not rejection.
		if round.Prevotes.Nil > round.Prevotes.Total/2 && len(round.Prevotes.Block) > 0 {
			return []Finding{{
				ID:       "proposal_rejected",
				Severity: Warning,
				Detail:   fmt.Sprintf("WAL round %d at height %d: %d of %d validators voted nil while others voted for a block — proposal rejected by majority (confidence: medium — could be gossip lag)", round.Round, ws.Height, round.Prevotes.Nil, round.Prevotes.Total),
				Source:   "data_dir",
			}}, nil
		}
	}
	return nil, nil
}

func checkStateMismatch(ctx *Context) ([]Finding, error) {
	ov, err := ctx.dataOverview.Get()
	if err != nil {
		return nil, err
	}
	if ov.BlockIDMatch == nil || *ov.BlockIDMatch {
		return nil, nil
	}
	return []Finding{{
		ID:       "state_mismatch",
		Severity: Critical,
		Detail:   "state.LastBlockID does not match blockstore — possible corruption or incomplete rollback",
		Source:   "data_dir",
	}}, nil
}
