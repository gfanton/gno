package nodedata

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/bft/state"
	dbm "github.com/gnolang/gno/tm2/pkg/db"
)

// TxHashScanWindow is the maximum number of blocks to scan backward
// when searching for a transaction by hash.
const TxHashScanWindow = 10000

// BlockDetail holds information about a single block.
type BlockDetail struct {
	Height     int64       `json:"height"`
	Time       string      `json:"time"`
	ChainID    string      `json:"chain_id"`
	NumTxs     int64       `json:"num_txs"`
	TotalTxs   int64       `json:"total_txs"`
	Proposer   string      `json:"proposer"`
	AppHash    string      `json:"app_hash"`
	TxResults  []TxResult  `json:"tx_results,omitempty"`
	Validators []Validator `json:"validators,omitempty"`

	// Block identity
	BlockID         string `json:"block_id,omitempty"`
	LastBlockID     string `json:"last_block_id,omitempty"`
	LastResultsHash string `json:"last_results_hash,omitempty"`
	DataHash        string `json:"data_hash,omitempty"`
	ValidatorsHash  string `json:"validators_hash,omitempty"`
	ConsensusHash   string `json:"consensus_hash,omitempty"`
}

// TxResult holds the outcome of a single transaction.
type TxResult struct {
	Index     int    `json:"index"`
	Hash      string `json:"hash,omitempty"`
	Type      string `json:"type,omitempty"`
	Sender    string `json:"sender,omitempty"`
	GasWanted int64  `json:"gas_wanted,omitempty"`
	Success   bool   `json:"success"`
	GasUsed   int64  `json:"gas_used"`
	Log       string `json:"log,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Validator holds address and voting power for a block's validator set.
type Validator struct {
	Address     string `json:"address"`
	VotingPower int64  `json:"voting_power"`
}

// Block loads a block and its associated data (ABCI responses, validators)
// from the local database. It recovers from panics that TM2 methods may
// produce on corrupt or missing data.
func (d *DataDir) Block(height int64) (detail *BlockDetail, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("corrupt block data at height %d: %v", height, r)
		}
	}()

	block := d.blockStore.LoadBlock(height)
	if block == nil {
		return nil, fmt.Errorf("block not found at height %d", height)
	}

	meta := d.blockStore.LoadBlockMeta(height)

	detail = &BlockDetail{
		Height:   block.Height,
		Time:     block.Time.UTC().Format("2006-01-02T15:04:05Z"),
		ChainID:  block.ChainID,
		NumTxs:   block.NumTxs,
		TotalTxs: block.TotalTxs,
		Proposer: block.ProposerAddress.String(),
		AppHash:  hex.EncodeToString(block.AppHash),

		LastBlockID:     block.LastBlockID.String(),
		LastResultsHash: hex.EncodeToString(block.LastResultsHash),
		DataHash:        hex.EncodeToString(block.DataHash),
		ValidatorsHash:  hex.EncodeToString(block.ValidatorsHash),
		ConsensusHash:   hex.EncodeToString(block.ConsensusHash),
	}

	if meta != nil {
		detail.BlockID = meta.BlockID.String()
	}

	// ---- ABCI Responses (loaded safely to avoid os.Exit on decode failure)
	abciResp, abciErr := loadABCIResponsesSafe(d.stateDB, height)
	if abciErr == nil && abciResp != nil {
		detail.TxResults = make([]TxResult, len(abciResp.DeliverTxs))
		for i, dtx := range abciResp.DeliverTxs {
			tr := TxResult{
				Index:   i,
				Success: dtx.IsOK(),
				GasUsed: dtx.GasUsed,
				Log:     dtx.Log,
			}
			// Error is an interface; nil-check before calling .Error()
			if dtx.Error != nil {
				tr.Error = dtx.Error.Error()
			}
			// Merge decoded tx summary if the raw tx bytes are available.
			if i < len(block.Data.Txs) {
				if summary, err := DecodeTxSummary(block.Data.Txs[i], i); err == nil {
					tr.Hash = summary.Hash
					tr.Type = summary.Type
					tr.Sender = summary.Sender
					tr.GasWanted = summary.GasWanted
				}
			}
			detail.TxResults[i] = tr
		}
	}

	// ---- Validators
	valSet, valErr := state.LoadValidators(d.stateDB, height)
	if valErr == nil && valSet != nil {
		detail.Validators = make([]Validator, len(valSet.Validators))
		for i, v := range valSet.Validators {
			detail.Validators[i] = Validator{
				Address:     v.Address.String(),
				VotingPower: v.VotingPower,
			}
		}
	}

	return detail, nil
}

// TxByIndex loads a single transaction by block height and index, returning
// the fully decoded payload merged with ABCI response data.
func (d *DataDir) TxByIndex(height int64, index int) (detail *TxDetail, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("corrupt data at height %d: %v", height, r)
		}
	}()

	block := d.blockStore.LoadBlock(height)
	if block == nil {
		return nil, fmt.Errorf("block not found at height %d", height)
	}
	if index < 0 || index >= len(block.Data.Txs) {
		return nil, fmt.Errorf("tx index %d out of range (block %d has %d txs)", index, height, len(block.Data.Txs))
	}

	detail, err = DecodeTxDetail(block.Data.Txs[index], height, index)
	if err != nil {
		return nil, err
	}

	// Merge ABCI response data if available.
	abciResp, abciErr := loadABCIResponsesSafe(d.stateDB, height)
	if abciErr == nil && abciResp != nil && index < len(abciResp.DeliverTxs) {
		dtx := abciResp.DeliverTxs[index]
		detail.Success = dtx.IsOK()
		detail.GasUsed = dtx.GasUsed
		detail.Log = dtx.Log
		if dtx.Error != nil {
			detail.Error = dtx.Error.Error()
		}
	}

	return detail, nil
}

// TxByHash scans blocks backward to find a transaction by its hash.
// It searches at most maxBlocks from the chain tip. Returns an error if
// the transaction is not found within the search window.
func (d *DataDir) TxByHash(hashHex string, maxBlocks int64) (detail *TxDetail, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic during tx hash search: %v", r)
		}
	}()

	targetHash, err := hex.DecodeString(hashHex)
	if err != nil {
		return nil, fmt.Errorf("invalid hash %q: %w", hashHex, err)
	}

	tip := d.blockStore.Height()
	start := tip
	end := tip - maxBlocks + 1
	if end < 1 {
		end = 1
	}

	for h := start; h >= end; h-- {
		block := d.blockStore.LoadBlock(h)
		if block == nil {
			continue
		}
		for i, tx := range block.Data.Txs {
			if !bytes.Equal(tx.Hash(), targetHash) {
				continue
			}
			return d.TxByIndex(h, i)
		}
	}

	return nil, fmt.Errorf("tx %s not found in last %d blocks (%d..%d)", hashHex, maxBlocks, end, start)
}

// loadABCIResponsesSafe loads ABCI responses without calling os.Exit on
// decode failure. The upstream state.LoadABCIResponses calls osm.Exit
// (which is os.Exit) when amino.Unmarshal fails, making it unsafe for
// diagnostic tools that must survive corrupt data.
func loadABCIResponsesSafe(db dbm.DB, height int64) (*state.ABCIResponses, error) {
	buf, err := db.Get(state.CalcABCIResponsesKey(height))
	if err != nil {
		return nil, fmt.Errorf("reading ABCI responses: %w", err)
	}
	if buf == nil {
		return nil, fmt.Errorf("no ABCI responses at height %d", height)
	}

	resp := new(state.ABCIResponses)
	if err := amino.Unmarshal(buf, resp); err != nil {
		return nil, fmt.Errorf("decoding ABCI responses at height %d: %w", height, err)
	}
	return resp, nil
}
