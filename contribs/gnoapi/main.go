package main

import (
	"context"
	"flag"
	"net/http"
	"os"

	"github.com/gnolang/gno/tm2/pkg/commands"
)

type apiCfg struct {
	remote   string
	listener string
}

var defaultApiOptions = &apiCfg{
	listener: "127.0.0.1:8585",
	remote:   "127.0.0.1:36657",
}

func main() {
	cfg := &apiCfg{}

	stdio := commands.NewDefaultIO()
	cmd := commands.NewCommand(
		commands.Metadata{
			Name:       "gnoapi",
			ShortUsage: "gnoapi [flags] [path ...]",
			ShortHelp:  "proxy node rpc api",
			LongHelp:   `gnoapi`,
		},
		cfg,
		func(_ context.Context, args []string) error {
			return execApi(cfg, args, stdio)
		})

	cmd.Execute(context.Background(), os.Args[1:])
}

func (c *apiCfg) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(
		&c.listener,
		"listen",
		defaultApiOptions.listener,
		"listener adress",
	)

	fs.StringVar(
		&c.listener,
		"remote",
		defaultApiOptions.remote,
		"remote node adress",
	)

}

func execApi(cfg *apiCfg, args []string, io commands.IO) error {
	server := http.Server{}

	return nil
}
