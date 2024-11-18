package core

import (
	"context"
	"encoding/hex"
	"fmt"

	ctypes "github.com/gnolang/gno/tm2/pkg/bft/rpc/core/types"
	sm "github.com/gnolang/gno/tm2/pkg/bft/state"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Tx allows you to query the transaction results. `nil` could mean the
// transaction is in the mempool, invalidated, or was not sent in the first
// place
func Tx(ctx context.Context, hash []byte) (*ctypes.ResultTx, error) {
	ctx, span := tracer().Start(ctx, "tx",
		trace.WithAttributes(attribute.String("hash", hex.EncodeToString(hash))))
	defer span.End()

	// Get the result index from storage, if any
	resultIndex, err := sm.LoadTxResultIndex(stateDB, hash)
	if err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.Int("resultsTxIndex", int(resultIndex.TxIndex)))

	// Sanity check the block height
	height, err := getHeight(blockStore.Height(), &resultIndex.BlockNum)
	if err != nil {
		return nil, err
	}

	span.SetAttributes(attribute.Int("resultsHeight", int(height)))

	// Load the block
	block := blockStore.LoadBlock(height)
	numTxs := len(block.Txs)

	if int(resultIndex.TxIndex) > numTxs || numTxs == 0 {
		return nil, fmt.Errorf(
			"unable to get block transaction for block %d, index %d",
			resultIndex.BlockNum,
			resultIndex.TxIndex,
		)
	}

	rawTx := block.Txs[resultIndex.TxIndex]

	// Fetch the block results
	blockResults, err := sm.LoadABCIResponses(stateDB, resultIndex.BlockNum)
	if err != nil {
		return nil, fmt.Errorf("unable to load block results, %w", err)
	}

	// Grab the block deliver response
	if len(blockResults.DeliverTxs) < int(resultIndex.TxIndex) {
		return nil, fmt.Errorf(
			"unable to get deliver result for block %d, index %d",
			resultIndex.BlockNum,
			resultIndex.TxIndex,
		)
	}

	deliverResponse := blockResults.DeliverTxs[resultIndex.TxIndex]

	// Craft the response
	return &ctypes.ResultTx{
		Hash:     hash,
		Height:   resultIndex.BlockNum,
		Index:    resultIndex.TxIndex,
		TxResult: deliverResponse,
		Tx:       rawTx,
	}, nil
}
