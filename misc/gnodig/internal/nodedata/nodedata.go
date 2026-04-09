package nodedata

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gnolang/gno/tm2/pkg/bft/state"
	bftstore "github.com/gnolang/gno/tm2/pkg/bft/store"
	dbm "github.com/gnolang/gno/tm2/pkg/db"
	_ "github.com/gnolang/gno/tm2/pkg/db/pebbledb"
)

// DataDir provides typed access to a gnoland node's data directory.
type DataDir struct {
	path       string
	blockDB    dbm.DB
	stateDB    dbm.DB
	blockStore *bftstore.BlockStore

	appMu     sync.Mutex
	appStores *appStores
}

// Overview holds summary information about the chain state.
type Overview struct {
	ChainID           string   `json:"chain_id"`
	LatestHeight      int64    `json:"latest_height"`
	LatestBlockTime   string   `json:"latest_block_time"`
	BlockStoreHeight  int64    `json:"block_store_height"`
	NumValidators     int      `json:"num_validators"`
	Validators        []string `json:"validators,omitempty"`
	BlockstoreAppHash string   `json:"blockstore_app_hash,omitempty"`
	WALHeight         int64    `json:"wal_height,omitempty"`

	// Consensus state fields from state.db
	StateAppHash     string `json:"state_app_hash,omitempty"`
	LastBlockID      string `json:"last_block_id,omitempty"`
	LastResultsHash  string `json:"last_results_hash,omitempty"`
	LastBlockTotalTx int64  `json:"last_block_total_tx,omitempty"`

	// Consistency: state.LastBlockID vs actual block in blockstore
	BlockIDMatch      *bool  `json:"block_id_match,omitempty"`
	BlockstoreBlockID string `json:"blockstore_block_id,omitempty"`
}

// ErrDatabaseLocked indicates the database is locked by another process
// (typically a running gnoland node). The node must be stopped before
// offline data access is possible.
var ErrDatabaseLocked = errors.New("database locked by another process (is the node running?)")

// isLockError checks whether an error is a PebbleDB directory lock failure.
func isLockError(err error) bool {
	if err == nil {
		return false
	}
	// PebbleDB lock error messages contain "lock" (varies by platform).
	return strings.Contains(strings.ToLower(err.Error()), "lock")
}

// Open opens a gnoland data directory and initializes access to the
// block store and state databases. The caller must call Close when done.
// Returns ErrDatabaseLocked if the database is locked by a running node.
func Open(path string) (*DataDir, error) {
	dbDir := filepath.Join(path, "db")
	if _, err := os.Stat(dbDir); err != nil {
		return nil, fmt.Errorf("data directory not accessible: %w", err)
	}

	blockDB, err := dbm.NewDB("blockstore", dbm.PebbleDBBackend, dbDir)
	if err != nil {
		if isLockError(err) {
			return nil, fmt.Errorf("opening blockstore db: %w", ErrDatabaseLocked)
		}
		return nil, fmt.Errorf("opening blockstore db: %w", err)
	}

	stateDB, err := dbm.NewDB("state", dbm.PebbleDBBackend, dbDir)
	if err != nil {
		blockDB.Close()
		if isLockError(err) {
			return nil, fmt.Errorf("opening state db: %w", ErrDatabaseLocked)
		}
		return nil, fmt.Errorf("opening state db: %w", err)
	}

	blockStore := bftstore.NewBlockStore(blockDB)

	return &DataDir{
		path:       path,
		blockDB:    blockDB,
		stateDB:    stateDB,
		blockStore: blockStore,
	}, nil
}

// Close releases all database resources.
func (d *DataDir) Close() error {
	d.appMu.Lock()
	defer d.appMu.Unlock()

	var errs []error
	if d.blockDB != nil {
		if err := d.blockDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("blockstore db: %w", err))
		}
	}
	if d.stateDB != nil {
		if err := d.stateDB.Close(); err != nil {
			errs = append(errs, fmt.Errorf("state db: %w", err))
		}
	}
	if d.appStores != nil {
		if err := d.appStores.Close(); err != nil {
			errs = append(errs, fmt.Errorf("app db: %w", err))
		}
	}
	return errors.Join(errs...)
}

// BlockStore returns the underlying block store for direct access.
func (d *DataDir) BlockStore() *bftstore.BlockStore {
	return d.blockStore
}

// Overview returns summary information about the chain state.
// It recovers from panics that TM2 state loading may produce on corrupt data.
func (d *DataDir) Overview() (ov *Overview, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic loading state: %v", r)
		}
	}()

	s := state.LoadState(d.stateDB)
	if s.IsEmpty() {
		return nil, fmt.Errorf("state database is empty")
	}

	ov = &Overview{
		ChainID:          s.ChainID,
		LatestHeight:     s.LastBlockHeight,
		LatestBlockTime:  s.LastBlockTime.UTC().Format("2006-01-02T15:04:05Z"),
		BlockStoreHeight: d.blockStore.Height(),

		StateAppHash:     hex.EncodeToString(s.AppHash),
		LastBlockID:      s.LastBlockID.String(),
		LastResultsHash:  hex.EncodeToString(s.LastResultsHash),
		LastBlockTotalTx: s.LastBlockTotalTx,
	}

	if s.Validators != nil {
		ov.NumValidators = s.Validators.Size()
		ov.Validators = make([]string, s.Validators.Size())
		for i, v := range s.Validators.Validators {
			ov.Validators[i] = v.Address.String()
		}
	}

	// Last app hash from the most recent block.
	lastBlock := d.blockStore.LoadBlock(ov.BlockStoreHeight)
	if lastBlock != nil {
		ov.BlockstoreAppHash = hex.EncodeToString(lastBlock.AppHash)
	}

	// Consistency check: state.LastBlockID vs actual block in blockstore.
	meta := d.blockStore.LoadBlockMeta(s.LastBlockHeight)
	if meta != nil {
		match := s.LastBlockID.Equals(meta.BlockID)
		ov.BlockIDMatch = &match
		ov.BlockstoreBlockID = meta.BlockID.String()
	}

	// Check WAL for uncommitted heights.
	walDir := filepath.Join(d.path, "wal", "cs.wal")
	if walHeight, err := scanWALMaxHeight(walDir); err == nil && walHeight > 0 {
		ov.WALHeight = walHeight
	}

	return ov, nil
}
