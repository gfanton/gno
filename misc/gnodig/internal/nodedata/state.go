package nodedata

import (
	"fmt"
	"strings"

	"github.com/gnolang/gno/gno.land/pkg/gnoland"
	"github.com/gnolang/gno/tm2/pkg/amino"
	"github.com/gnolang/gno/tm2/pkg/crypto"
	dbm "github.com/gnolang/gno/tm2/pkg/db"
	"github.com/gnolang/gno/tm2/pkg/std"
	"github.com/gnolang/gno/tm2/pkg/store/dbadapter"
	"github.com/gnolang/gno/tm2/pkg/store/iavl"
	"github.com/gnolang/gno/tm2/pkg/store/rootmulti"
	"github.com/gnolang/gno/tm2/pkg/store/types"
)

// Force amino registration for GnoAccount deserialization.
var _ = gnoland.Package

// Store key names matching those in gno.land/pkg/gnoland/app.go.
const (
	mainStoreName = "main"
	baseStoreName = "base"
)

// AccountResult holds the result of an account state query.
type AccountResult struct {
	Height   int64  `json:"height"`
	Address  string `json:"address"`
	Exists   bool   `json:"exists"`
	Coins    any    `json:"coins,omitempty"`
	Sequence uint64 `json:"sequence,omitempty"`
}

// PackageResult holds the result of a package state query.
type PackageResult struct {
	Height   int64         `json:"height"`
	Path     string        `json:"path"`
	Exists   bool          `json:"exists"`
	NumFiles int           `json:"num_files,omitempty"`
	Files    []PackageFile `json:"files,omitempty"`
}

// PackageFile holds the name and size of a file in a package.
type PackageFile struct {
	Name string `json:"name"`
	Size int    `json:"size"`
}

// ---- App Store

// appStores bundles the opened DB, multiStore, and store keys for querying.
type appStores struct {
	db      dbm.DB
	ms      types.CommitMultiStore
	mainKey types.StoreKey
	baseKey types.StoreKey
}

func (a *appStores) Close() error {
	if a.db != nil {
		return a.db.Close()
	}
	return nil
}

// openAppStore opens the gnolang app database and creates a multiStore
// with the same layout as the gno.land app.
func openAppStore(dataDir string) (*appStores, error) {
	dbDir := dataDir + "/db"
	db, err := dbm.NewDB("gnolang", dbm.PebbleDBBackend, dbDir)
	if err != nil {
		return nil, fmt.Errorf("opening gnolang db: %w", err)
	}

	ms := rootmulti.NewMultiStore(db)

	mainKey := types.NewStoreKey(mainStoreName)
	baseKey := types.NewStoreKey(baseStoreName)

	// Mount stores matching the gno.land app layout.
	ms.MountStoreWithDB(mainKey, iavl.StoreConstructor, db)
	ms.MountStoreWithDB(baseKey, dbadapter.StoreConstructor, db)

	return &appStores{
		db:      db,
		ms:      ms,
		mainKey: mainKey,
		baseKey: baseKey,
	}, nil
}

// loadVersion loads either the latest version or a specific height.
func (a *appStores) loadVersion(height int64) error {
	if height == 0 {
		return a.ms.LoadLatestVersion()
	}
	return a.ms.LoadVersion(height)
}

// getAppStores returns the cached app stores, opening them lazily on first call.
// Caller must hold d.appMu.
func (d *DataDir) getAppStoresLocked() (*appStores, error) {
	if d.appStores != nil {
		return d.appStores, nil
	}
	app, err := openAppStore(d.path)
	if err != nil {
		return nil, err
	}
	d.appStores = app
	return app, nil
}

// ---- Account Queries

// AccountQuery retrieves account state at the given height. Height 0 means
// latest. The address should be a bech32 address string.
func (d *DataDir) AccountQuery(height int64, address string) (_ *AccountResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic querying account state: %v", r)
		}
	}()

	addr, err := crypto.AddressFromBech32(address)
	if err != nil {
		return nil, fmt.Errorf("invalid address %q: %w", address, err)
	}

	d.appMu.Lock()
	defer d.appMu.Unlock()

	app, err := d.getAppStoresLocked()
	if err != nil {
		return nil, err
	}

	if err := app.loadVersion(height); err != nil {
		return nil, fmt.Errorf("loading version: %w", err)
	}

	mainStore := app.ms.GetCommitStore(app.mainKey)

	// Account key format: "/a/" + address bytes (from tm2/pkg/sdk/auth/consts.go)
	key := append([]byte("/a/"), addr.Bytes()...)
	bz := mainStore.Get(key)

	result := &AccountResult{
		Height:  app.ms.LastCommitID().Version,
		Address: address,
	}

	if bz == nil {
		return result, nil
	}

	result.Exists = true

	var acc std.Account
	if err := amino.Unmarshal(bz, &acc); err != nil {
		return nil, fmt.Errorf("decoding account: %w", err)
	}

	result.Coins = acc.GetCoins()
	result.Sequence = acc.GetSequence()

	return result, nil
}

// ---- Package Queries

// PackageQuery retrieves package metadata at the given height. Height 0 means
// latest. The pkgPath should be a gno package path like "gno.land/r/demo/boards"
// or a stdlib name like "bufio".
func (d *DataDir) PackageQuery(height int64, pkgPath string) (_ *PackageResult, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic querying package state: %v", r)
		}
	}()

	d.appMu.Lock()
	defer d.appMu.Unlock()

	app, err := d.getAppStoresLocked()
	if err != nil {
		return nil, err
	}

	if err := app.loadVersion(height); err != nil {
		return nil, fmt.Errorf("loading version: %w", err)
	}

	mainStore := app.ms.GetCommitStore(app.mainKey)

	// Package key format: "pkg:" + path, with stdlib prefix "_/"
	// (from gnovm/pkg/gnolang/store.go)
	key := []byte(backendPackagePathKey(pkgPath))
	bz := mainStore.Get(key)

	result := &PackageResult{
		Height: app.ms.LastCommitID().Version,
		Path:   pkgPath,
	}

	if bz == nil {
		return result, nil
	}

	result.Exists = true

	var mpkg std.MemPackage
	if err := amino.Unmarshal(bz, &mpkg); err != nil {
		return nil, fmt.Errorf("decoding package: %w", err)
	}

	result.NumFiles = len(mpkg.Files)
	result.Files = make([]PackageFile, len(mpkg.Files))
	for i, f := range mpkg.Files {
		result.Files[i] = PackageFile{
			Name: f.Name,
			Size: len(f.Body),
		}
	}

	return result, nil
}

// ListPackagePaths returns up to limit package paths from the IAVL store.
func (d *DataDir) ListPackagePaths(height int64, limit int) (_ []string, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("panic listing package paths: %v", r)
		}
	}()

	d.appMu.Lock()
	defer d.appMu.Unlock()

	app, err := d.getAppStoresLocked()
	if err != nil {
		return nil, err
	}

	if err := app.loadVersion(height); err != nil {
		return nil, fmt.Errorf("loading version: %w", err)
	}

	mainStore := app.ms.GetCommitStore(app.mainKey)

	var paths []string
	start := []byte("pkg:")
	end := []byte("pkg;") // ';' is the byte after ':'
	iter := mainStore.Iterator(start, end)
	defer iter.Close()

	for ; iter.Valid() && len(paths) < limit; iter.Next() {
		paths = append(paths, decodeBackendPackagePathKey(string(iter.Key())))
	}

	return paths, nil
}

// ---- Key Format Helpers

// backendPackagePathKey reproduces the key format from gnovm/pkg/gnolang/store.go.
func backendPackagePathKey(path string) string {
	if isStdlib(path) {
		return "pkg:_/" + path
	}
	return "pkg:" + path
}

// decodeBackendPackagePathKey reverses backendPackagePathKey.
func decodeBackendPackagePathKey(key string) string {
	// Check stdlib prefix first (more specific).
	if rest, ok := strings.CutPrefix(key, "pkg:_/"); ok {
		return rest
	}
	if rest, ok := strings.CutPrefix(key, "pkg:"); ok {
		return rest
	}
	return key
}

// isStdlib checks if a path is a standard library path (no domain with a dot).
func isStdlib(path string) bool {
	for i := 0; i < len(path); i++ {
		if path[i] == '/' {
			return true // no dot before first slash
		}
		if path[i] == '.' {
			return false // dot before first slash -> domain path
		}
	}
	return true // no slash at all -> stdlib
}
