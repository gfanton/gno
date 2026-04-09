package chainrpc

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tidwall/gjson"
)

// ---- Genesis Types

// GenesisSummary holds parsed genesis metadata.
type GenesisSummary struct {
	ChainID         string                 `json:"chain_id"`
	GenesisTime     string                 `json:"genesis_time"`
	Validators      []GenesisValidator     `json:"validators"`
	ConsensusParams GenesisConsensusParams `json:"consensus_params"`
	BalanceCount    int                    `json:"balance_count"`
	Cached          bool                   `json:"cached"`
	CachePath       string                 `json:"cache_path,omitempty"`
}

// GenesisValidator holds a genesis validator entry.
type GenesisValidator struct {
	Address string `json:"address"`
	Power   string `json:"power"`
	Name    string `json:"name"`
}

// GenesisConsensusParams holds consensus parameters.
type GenesisConsensusParams struct {
	MaxGas       string `json:"max_gas"`
	MaxTxBytes   string `json:"max_tx_bytes"`
	MaxDataBytes string `json:"max_data_bytes"`
}

// GenesisBalanceResult holds the result of a genesis balance lookup.
type GenesisBalanceResult struct {
	Address string `json:"address"`
	Balance string `json:"balance,omitempty"`
	Found   bool   `json:"found"`
	ChainID string `json:"chain_id"`
}

// ---- Cache Helpers

func genesisCachePath(cacheDir, chainID string) string {
	return filepath.Join(cacheDir, "chains", chainID, "genesis.json")
}

func genesisIsCached(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Size() > 0
}

// ---- Download

// genesisDownloadTimeout is longer than the default client timeout because
// genesis files can be very large (185MB+ on gnoland1).
const genesisDownloadTimeout = 5 * time.Minute

// downloadGenesis fetches the genesis document from the RPC endpoint and
// streams it to destPath. Serialized by c.genesisMu to prevent concurrent
// writes to the same cache file from the same client.
func (c *Client) downloadGenesis(ctx context.Context, destPath string) error {
	c.genesisMu.Lock()
	defer c.genesisMu.Unlock()

	// Double-check after acquiring lock -- another goroutine may have downloaded it
	if genesisIsCached(destPath) {
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
		return fmt.Errorf("create cache dir: %w", err)
	}

	tmpPath := destPath + ".tmp"

	u := c.rpcURL + "/genesis"
	dlCtx, cancel := context.WithTimeout(ctx, genesisDownloadTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(dlCtx, http.MethodGet, u, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := (&http.Client{}).Do(req)
	if err != nil {
		return fmt.Errorf("fetch genesis: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch genesis: HTTP %d", resp.StatusCode)
	}

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}

	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("download genesis: %w", err)
	}
	if err := f.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	return os.Rename(tmpPath, destPath)
}

// ---- Parsing

// extractGenesisMetadata extracts chain metadata from a gjson genesis result.
// Shared between parseGenesisSummary (small files) and parseSummaryFromFile (large files).
func extractGenesisMetadata(genesis gjson.Result) *GenesisSummary {
	summary := &GenesisSummary{
		ChainID:     genesis.Get("chain_id").String(),
		GenesisTime: genesis.Get("genesis_time").String(),
	}

	genesis.Get("validators").ForEach(func(_, v gjson.Result) bool {
		summary.Validators = append(summary.Validators, GenesisValidator{
			Address: v.Get("address").String(),
			Power:   v.Get("power").String(),
			Name:    v.Get("name").String(),
		})
		return true
	})

	cp := genesis.Get("consensus_params")
	summary.ConsensusParams = GenesisConsensusParams{
		MaxGas:       cp.Get("Block.MaxGas").String(),
		MaxTxBytes:   cp.Get("Block.MaxTxBytes").String(),
		MaxDataBytes: cp.Get("Block.MaxDataBytes").String(),
	}

	return summary
}

// resolveGenesisRoot unwraps the RPC response wrapper if present.
func resolveGenesisRoot(root gjson.Result) gjson.Result {
	if g := root.Get("result.genesis"); g.Exists() {
		return g
	}
	return root
}

// parseGenesisSummary extracts metadata from genesis JSON. Works on both
// raw genesis and RPC-wrapped genesis ({"result":{"genesis":{...}}}).
func parseGenesisSummary(data []byte) (*GenesisSummary, error) {
	genesis := resolveGenesisRoot(gjson.ParseBytes(data))
	summary := extractGenesisMetadata(genesis)

	// For small files we can count balances directly from the parsed JSON
	balances := genesis.Get("app_state.balances")
	if balances.IsArray() {
		summary.BalanceCount = len(balances.Array())
	}

	return summary, nil
}

// parseSummaryFromFile reads a cached genesis file and extracts metadata.
// For small files (< 1MB), reads the whole thing. For large files, reads
// only the header for metadata and scans for balance count.
func parseSummaryFromFile(path string) (*GenesisSummary, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	// For small files (tests), read the whole thing
	if info.Size() < 1<<20 {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		return parseGenesisSummary(data)
	}

	// For large files, read the header (64KB) for metadata
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	header := make([]byte, 64*1024)
	n, err := f.Read(header)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil, fmt.Errorf("read genesis header: %w", err)
	}
	if n == 0 {
		return nil, fmt.Errorf("genesis file is empty")
	}

	genesis := resolveGenesisRoot(gjson.ParseBytes(header[:n]))
	summary := extractGenesisMetadata(genesis)

	// Count balance entries by scanning for "g1" address patterns
	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("seek genesis: %w", err)
	}
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	count := 0
	inBalances := false
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.Contains(line, `"balances"`) {
			inBalances = true
			continue
		}
		if inBalances {
			if line == "]" || line == "]," {
				break
			}
			if strings.HasPrefix(line, `"g1`) {
				count++
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("scan genesis: %w", err)
	}
	summary.BalanceCount = count

	return summary, nil
}

// lookupBalance scans a cached genesis file for a specific address balance.
// Balances are stored as "address=amountugnot" strings in the JSON array.
func lookupBalance(path, address string) (string, bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", false, err
	}
	defer f.Close()

	// Scan for the address pattern: "address=amount"
	target := `"` + address + "="
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)

	for scanner.Scan() {
		line := scanner.Text()
		idx := strings.Index(line, target)
		if idx < 0 {
			continue
		}

		// Extract the balance value after "address=
		after := line[idx+len(target):]
		end := strings.IndexByte(after, '"')
		if end < 0 {
			continue
		}
		if end > 0 {
			return after[:end], true, nil
		}
	}

	return "", false, scanner.Err()
}

// ---- Public API

// ensureGenesisCached fetches the chain ID and downloads genesis if needed.
func (c *Client) ensureGenesisCached(ctx context.Context, cacheDir string) (cachePath, chainID string, err error) {
	status, err := c.Status(ctx)
	if err != nil {
		return "", "", fmt.Errorf("fetch status for chain_id: %w", err)
	}
	chainID = status.Get("node_info.network").String()
	if chainID == "" {
		return "", "", fmt.Errorf("could not determine chain_id from status")
	}

	cachePath = genesisCachePath(cacheDir, chainID)
	if !genesisIsCached(cachePath) {
		if err := c.downloadGenesis(ctx, cachePath); err != nil {
			return "", "", fmt.Errorf("download genesis: %w", err)
		}
	}
	return cachePath, chainID, nil
}

// FetchGenesisSummary returns genesis metadata. Downloads and caches the
// genesis file on first call. Subsequent calls read from cache.
func (c *Client) FetchGenesisSummary(ctx context.Context, cacheDir string) (*GenesisSummary, error) {
	cachePath, _, err := c.ensureGenesisCached(ctx, cacheDir)
	if err != nil {
		return nil, err
	}

	summary, err := parseSummaryFromFile(cachePath)
	if err != nil {
		return nil, err
	}

	summary.Cached = true
	summary.CachePath = cachePath
	return summary, nil
}

// LookupGenesisBalance looks up an address balance in the cached genesis.
func (c *Client) LookupGenesisBalance(ctx context.Context, cacheDir, address string) (*GenesisBalanceResult, error) {
	cachePath, chainID, err := c.ensureGenesisCached(ctx, cacheDir)
	if err != nil {
		return nil, err
	}

	balance, found, err := lookupBalance(cachePath, address)
	if err != nil {
		return nil, err
	}

	return &GenesisBalanceResult{
		Address: address,
		Balance: balance,
		Found:   found,
		ChainID: chainID,
	}, nil
}

// FetchGenesisSummaryFromCache reads genesis from cache only, without RPC.
// Returns nil, nil if not cached.
func FetchGenesisSummaryFromCache(cacheDir, chainID string) (*GenesisSummary, error) {
	cachePath := genesisCachePath(cacheDir, chainID)
	if !genesisIsCached(cachePath) {
		return nil, nil
	}

	summary, err := parseSummaryFromFile(cachePath)
	if err != nil {
		return nil, err
	}
	summary.Cached = true
	summary.CachePath = cachePath
	return summary, nil
}
