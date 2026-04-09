package chainrpc

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestParseGenesisSummary(t *testing.T) {
	// Minimal genesis JSON matching the real structure
	genesis := `{
		"genesis_time": "2026-03-16T09:00:00Z",
		"chain_id": "testchain",
		"consensus_params": {
			"Block": {"MaxTxBytes": "1000000", "MaxDataBytes": "2000000", "MaxGas": "3000000000"},
			"Validator": {"PubKeyTypeURLs": ["/tm.PubKeyEd25519"]}
		},
		"validators": [
			{"address": "g1abc", "pub_key": {}, "power": "1", "name": "val01"},
			{"address": "g1def", "pub_key": {}, "power": "1", "name": "val02"}
		],
		"app_state": {
			"@type": "/gno.GenesisState",
			"balances": ["g1aaa=100ugnot", "g1bbb=200ugnot", "g1ccc=300ugnot"]
		}
	}`

	summary, err := parseGenesisSummary([]byte(genesis))
	require.NoError(t, err)
	require.Equal(t, "testchain", summary.ChainID)
	require.Equal(t, "2026-03-16T09:00:00Z", summary.GenesisTime)
	require.Len(t, summary.Validators, 2)
	require.Equal(t, "g1abc", summary.Validators[0].Address)
	require.Equal(t, "3000000000", summary.ConsensusParams.MaxGas)
	require.Equal(t, 3, summary.BalanceCount)
}

func TestParseGenesisSummary_RPCWrapped(t *testing.T) {
	// RPC-wrapped genesis (from /genesis endpoint)
	genesis := `{"result":{"genesis":{
		"genesis_time": "2026-03-16T09:00:00Z",
		"chain_id": "wrapped-chain",
		"validators": [{"address": "g1xxx", "power": "10", "name": "node0"}],
		"consensus_params": {"Block": {"MaxGas": "5000000"}},
		"app_state": {"balances": ["g1aaa=100ugnot"]}
	}}}`

	summary, err := parseGenesisSummary([]byte(genesis))
	require.NoError(t, err)
	require.Equal(t, "wrapped-chain", summary.ChainID)
	require.Len(t, summary.Validators, 1)
	require.Equal(t, 1, summary.BalanceCount)
}

func TestLookupBalance(t *testing.T) {
	genesis := `{"app_state":{"balances":["g1aaa=100ugnot","g1bbb=200ugnot","g1ccc=300ugnot"]}}`

	dir := t.TempDir()
	path := filepath.Join(dir, "genesis.json")
	require.NoError(t, os.WriteFile(path, []byte(genesis), 0o644))

	// Found
	balance, found, err := lookupBalance(path, "g1bbb")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "200ugnot", balance)

	// Not found
	_, found, err = lookupBalance(path, "g1zzz")
	require.NoError(t, err)
	require.False(t, found)
}

func TestLookupBalance_MultiLine(t *testing.T) {
	// Balance entries on separate lines (like real genesis files)
	genesis := `{
  "app_state": {
    "balances": [
      "g1aaa=100ugnot",
      "g1bbb=200ugnot",
      "g1ccc=300ugnot"
    ]
  }
}`

	dir := t.TempDir()
	path := filepath.Join(dir, "genesis.json")
	require.NoError(t, os.WriteFile(path, []byte(genesis), 0o644))

	balance, found, err := lookupBalance(path, "g1ccc")
	require.NoError(t, err)
	require.True(t, found)
	require.Equal(t, "300ugnot", balance)
}

func TestGenesisCache_HitMiss(t *testing.T) {
	dir := t.TempDir()
	chainID := "testchain"

	// Miss
	path := genesisCachePath(dir, chainID)
	require.False(t, genesisIsCached(path))

	// Write cache
	genesis := `{"genesis_time":"2026-03-16T09:00:00Z","chain_id":"testchain"}`
	require.NoError(t, os.MkdirAll(filepath.Dir(path), 0o755))
	require.NoError(t, os.WriteFile(path, []byte(genesis), 0o644))

	// Hit
	require.True(t, genesisIsCached(path))
}

func TestGenesisCachePath(t *testing.T) {
	path := genesisCachePath("/home/user/.debug", "gnoland")
	require.Equal(t, "/home/user/.debug/chains/gnoland/genesis.json", path)
}

func TestGenesisIsCached_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "genesis.json")

	// Empty file should not count as cached
	require.NoError(t, os.WriteFile(path, []byte{}, 0o644))
	require.False(t, genesisIsCached(path))
}
