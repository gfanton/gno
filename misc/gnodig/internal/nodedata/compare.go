package nodedata

import (
	"fmt"
	"slices"
	"strings"
)

// nodeBlock pairs a node path with its block data (or error).
type nodeBlock struct {
	path   string
	detail *BlockDetail
	err    error
}

// CompareResult holds the outcome of comparing the same block height across
// multiple nodes.
type CompareResult struct {
	Height            int64           `json:"height"`
	Nodes             []string        `json:"nodes"`
	AppHash           FieldComparison `json:"app_hash"`
	NumTxs            FieldComparison `json:"num_txs"`
	TxDiffs           []TxDiff        `json:"tx_diffs,omitempty"`
	ValidatorSetMatch bool            `json:"validator_set_match"`
}

// FieldComparison reports whether a single field matches across all nodes.
// When Match is true, Value contains the shared value.
// When Match is false, Values maps each node path to its value.
type FieldComparison struct {
	Match  bool           `json:"match"`
	Value  any            `json:"value,omitempty"`
	Values map[string]any `json:"values,omitempty"`
}

// TxDiff reports whether a single transaction matches across nodes.
type TxDiff struct {
	Index int            `json:"index"`
	Hash  string         `json:"hash,omitempty"`
	Match bool           `json:"match"`
	Diffs map[string]any `json:"diffs,omitempty"`
}

// CompareBlocks loads the block at height from each DataDir and compares
// appHash, numTxs, transaction results, and validator sets.
func CompareBlocks(dirs []*DataDir, height int64) (*CompareResult, error) {
	if len(dirs) < 2 {
		return nil, fmt.Errorf("compare requires at least 2 data dirs, got %d", len(dirs))
	}
	if len(dirs) > 5 {
		return nil, fmt.Errorf("compare supports at most 5 data dirs, got %d", len(dirs))
	}

	nodes := make([]nodeBlock, len(dirs))
	for i, dd := range dirs {
		detail, err := dd.Block(height)
		nodes[i] = nodeBlock{
			path:   dd.path,
			detail: detail,
			err:    err,
		}
	}

	result := &CompareResult{
		Height: height,
		Nodes:  make([]string, len(nodes)),
	}
	for i, n := range nodes {
		result.Nodes[i] = n.path
	}

	result.AppHash = compareField(nodes, func(nb nodeBlock) string {
		return nb.detail.AppHash
	})
	result.NumTxs = compareField(nodes, func(nb nodeBlock) int64 {
		return nb.detail.NumTxs
	})

	// ---- Tx Diffs
	maxTxs := 0
	for _, n := range nodes {
		if n.err == nil && len(n.detail.TxResults) > maxTxs {
			maxTxs = len(n.detail.TxResults)
		}
	}
	if maxTxs > 0 {
		result.TxDiffs = make([]TxDiff, maxTxs)
		for i := range maxTxs {
			result.TxDiffs[i] = compareTx(nodes, i)
		}
	}

	// ---- Validator Set
	result.ValidatorSetMatch = compareValidatorSets(nodes)

	return result, nil
}

// compareField compares a field value across all nodes using generics.
func compareField[T comparable](nodes []nodeBlock, extract func(nodeBlock) T) FieldComparison {
	var first T
	firstSet := false
	match := true

	for _, n := range nodes {
		if n.err != nil {
			match = false
			continue
		}
		v := extract(n)
		if !firstSet {
			first = v
			firstSet = true
			continue
		}
		if v != first {
			match = false
		}
	}

	if match && firstSet {
		return FieldComparison{Match: true, Value: first}
	}

	values := make(map[string]any, len(nodes))
	for _, n := range nodes {
		if n.err != nil {
			values[n.path] = "missing"
		} else {
			values[n.path] = extract(n)
		}
	}
	return FieldComparison{Match: false, Values: values}
}

// compareTx compares a single transaction at the given index across nodes.
func compareTx(nodes []nodeBlock, index int) TxDiff {
	diff := TxDiff{Index: index, Match: true}

	type txFields struct {
		gasUsed int64
		success bool
		errMsg  string
	}

	var first *txFields

	for _, n := range nodes {
		if n.err != nil || index >= len(n.detail.TxResults) {
			diff.Match = false
			continue
		}

		tr := n.detail.TxResults[index]
		if diff.Hash == "" {
			diff.Hash = tr.Hash
		}

		f := &txFields{
			gasUsed: tr.GasUsed,
			success: tr.Success,
			errMsg:  tr.Error,
		}

		if first == nil {
			first = f
			continue
		}

		if f.gasUsed != first.gasUsed || f.success != first.success || f.errMsg != first.errMsg {
			diff.Match = false
		}
	}

	// Build diff map lazily — only when mismatch found.
	if !diff.Match {
		diff.Diffs = make(map[string]any, len(nodes))
		for _, n := range nodes {
			if n.err != nil || index >= len(n.detail.TxResults) {
				diff.Diffs[n.path] = "missing"
				continue
			}
			tr := n.detail.TxResults[index]
			diff.Diffs[n.path] = map[string]any{
				"gas_used": tr.GasUsed,
				"success":  tr.Success,
				"error":    tr.Error,
			}
		}
	}

	return diff
}

// compareValidatorSets checks whether all nodes have the same validator set.
// It builds a canonical sorted string of "addr:votingPower" pairs for each
// node and compares them.
func compareValidatorSets(nodes []nodeBlock) bool {
	canonicalize := func(validators []Validator) string {
		parts := make([]string, len(validators))
		for i, v := range validators {
			parts[i] = fmt.Sprintf("%s:%d", v.Address, v.VotingPower)
		}
		slices.Sort(parts)
		return strings.Join(parts, ",")
	}

	first := ""
	firstSet := false
	for _, n := range nodes {
		if n.err != nil {
			return false
		}
		c := canonicalize(n.detail.Validators)
		if !firstSet {
			first = c
			firstSet = true
			continue
		}
		if c != first {
			return false
		}
	}

	return firstSet
}
