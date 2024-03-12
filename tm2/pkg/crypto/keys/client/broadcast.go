package client

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"

	"github.com/gnolang/gno/tm2/pkg/amino"
	abci "github.com/gnolang/gno/tm2/pkg/bft/abci/types"
	"github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	ctypes "github.com/gnolang/gno/tm2/pkg/bft/rpc/core/types"
	"github.com/gnolang/gno/tm2/pkg/commands"
	"github.com/gnolang/gno/tm2/pkg/std"
)

type BroadcastCfg struct {
	RootCfg *BaseCfg

	DryRun bool

	// internal
	tx *std.Tx
}

func NewBroadcastCmd(rootCfg *BaseCfg, io commands.IO) *commands.Command {
	cfg := &BroadcastCfg{
		RootCfg: rootCfg,
	}

	return commands.NewCommand(
		commands.Metadata{
			Name:       "broadcast",
			ShortUsage: "broadcast [flags] <file-name>",
			ShortHelp:  "broadcasts a signed document",
		},
		cfg,
		func(_ context.Context, args []string) error {
			return execBroadcast(cfg, args, commands.NewDefaultIO())
		},
	)
}

func (c *BroadcastCfg) RegisterFlags(fs *flag.FlagSet) {
	fs.BoolVar(
		&c.DryRun,
		"dry-run",
		false,
		"perform a dry-run broadcast",
	)
}

func execBroadcast(cfg *BroadcastCfg, args []string, io commands.IO) error {
	if len(args) != 1 {
		return flag.ErrHelp
	}
	filename := args[0]

	jsonbz, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading tx document file %s: %w", filename, err)
	}
	var tx std.Tx
	err = amino.UnmarshalJSON(jsonbz, &tx)
	if err != nil {
		return fmt.Errorf("unmarshaling tx json bytes: %w", err)
	}
	cfg.tx = &tx

	res, err := BroadcastHandler(cfg)
	if err != nil {
		return err
	}

	if res.CheckTx.IsErr() {
		return fmt.Errorf("transaction failed %#v\nlog %s", res, res.CheckTx.Log)
	} else if res.DeliverTx.IsErr() {
		return fmt.Errorf("transaction failed %#v\nlog %s", res, res.DeliverTx.Log)
	} else {
		io.Println(string(res.DeliverTx.Data))
		io.Println("OK!")
		io.Println("GAS WANTED:", res.DeliverTx.GasWanted)
		io.Println("GAS USED:  ", res.DeliverTx.GasUsed)
	}
	return nil
}

func BroadcastHandler(cfg *BroadcastCfg) (*ctypes.ResultBroadcastTxCommit, error) {
	if cfg.tx == nil {
		return nil, errors.New("invalid tx")
	}

	remote := cfg.RootCfg.Remote
	if remote == "" || remote == "y" {
		return nil, errors.New("missing remote url")
	}

	bz, err := amino.Marshal(cfg.tx)
	if err != nil {
		return nil, fmt.Errorf("remarshaling tx binary bytes: %w", err)
	}

	cli := client.NewHTTP(remote, "/websocket")

	if cfg.DryRun {
		return SimulateTx(cli, bz)
	}

	bres, err := cli.BroadcastTxCommit(bz)
	if err != nil {
		return nil, fmt.Errorf("broadcasting bytes: %w", err)
	}

	return bres, nil
}

func SimulateTx(cli client.ABCIClient, tx []byte) (*ctypes.ResultBroadcastTxCommit, error) {
	bres, err := cli.ABCIQuery(".app/simulate", tx)
	if err != nil {
		return nil, fmt.Errorf("simulate tx: %w", err)
	}

	var result abci.ResponseDeliverTx
	err = amino.Unmarshal(bres.Response.Value, &result)
	if err != nil {
		return nil, fmt.Errorf("unmarshaling simulate result: %w", err)
	}

	return &ctypes.ResultBroadcastTxCommit{
		DeliverTx: result,
	}, nil
}
