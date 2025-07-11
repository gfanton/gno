package main

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/gnolang/contribs/gnogenesis/internal/common"
	"github.com/gnolang/contribs/gnogenesis/internal/validator"
	"github.com/gnolang/gno/gno.land/pkg/integration"
	"github.com/gnolang/gno/gnovm/pkg/gnoenv"
	"github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	"github.com/gnolang/gno/tm2/pkg/bft/types"
	"github.com/gnolang/gno/tm2/pkg/commands"
	"github.com/gnolang/gno/tm2/pkg/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIntegrationTest(t *testing.T) {
	const (
		chainid     = "test"
		genesisTime = "1750402800" // Friday, June 20th 2025 09:00 GMT+0200 (Central European Summer Time)
	)

	ctx := context.Background()

	logger := log.NewTestingLogger(t)
	gnoroot := gnoenv.RootDir()

	genesisfile := filepath.Join(t.TempDir(), "genesis.json")
	cmd := newGenesisCmd(commands.NewTestIO())
	cmd.ParseAndRun(ctx, []string{"gnogenesis",
		"generate",
		"-chain-id", chainid,
		"-genesis-time", genesisTime,
		"-output-path", genesisfile,
	})

	cmd.ParseAndRun(ctx, []string{"gnogenesis",
		"validator", "add",
		"--genesis-path",
		tempGenesis.Name(),
		"--address",
		key.Address().String(),
		"--power",
		"-1", // invalid voting power
	})

	genesis, err := types.GenesisDocFromFile(genesisfile)
	require.NoError(t, err)

	key := common.GetDummyKey(t)

	// Create the command
	args :=

	err := genesis.Validate()
	require.NoError(t, err)

	cfg := integration.TestingMinimalNodeConfig(gnoroot)
	cfg.Genesis = genesis

	node, address := integration.TestingInMemoryNode(t, logger, cfg)
	defer node.Stop()

	cli, err := client.NewHTTPClient(address)
	require.NoError(t, err)

	s, err := cli.Status()
	require.NoError(t, err)

	assert.Equal(t, s.NodeInfo.Network, chainid)
	// ... ensure everything else is correctly setup
}
