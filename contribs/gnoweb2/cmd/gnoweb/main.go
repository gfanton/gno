package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	gnoweb "github.com/gnolang/gno/contribs/gnoweb2"
	"github.com/gnolang/gno/contribs/gnoweb2/pkg/service"
	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
	"github.com/gnolang/gno/gno.land/pkg/log"
	"github.com/gnolang/gno/tm2/pkg/bft/rpc/client"
	"github.com/gnolang/gno/tm2/pkg/commands"
	"github.com/yuin/goldmark"
	"go.uber.org/zap/zapcore"
)

type webCfg struct {
	listener string
	loglevel string
	remote   string
}

func main() {
	cfg := &webCfg{}

	stdio := commands.NewDefaultIO()
	cmd := commands.NewCommand(
		commands.Metadata{
			Name:       "gnodev",
			ShortUsage: "gnodev [flags] [path ...]",
			ShortHelp:  "runs an in-memory node and gno.land web server for development purposes.",
			LongHelp:   `The gnodev command starts an in-memory node and a gno.land web interface primarily for realm package development. It automatically loads the 'examples' directory and any additional specified paths.`,
		},
		cfg,
		func(_ context.Context, args []string) error {
			return execWeb(cfg, args, stdio)
		})

	cmd.Execute(context.Background(), os.Args[1:])
}

func (c *webCfg) RegisterFlags(fs *flag.FlagSet) {
	fs.StringVar(
		&c.listener,
		"listener",
		"127.0.0.1:8888",
		"gnoweb main listener",
	)

	fs.StringVar(
		&c.remote,
		"remote",
		"127.0.0.1:26657",
		"gnoland remote url",
	)

}

func execWeb(cfg *webCfg, args []string, io commands.IO) (err error) {
	ctx := context.TODO()

	zapLogger := log.NewZapConsoleLogger(os.Stdout, zapcore.DebugLevel)
	defer zapLogger.Sync()

	// Setup logger
	logger := log.ZapLoggerToSlog(zapLogger)

	md := goldmark.New()

	staticMeta := gnoweb.StaticMetadata{
		Assetspath: "/assets",
	}

	mux := http.NewServeMux()

	// Setup asset handler
	mux.Handle(staticMeta.Assetspath, gnoweb.AssetHandler())

	client, err := client.NewHTTPClient(cfg.remote)
	if err != nil {
		return fmt.Errorf("unable to create http client: %W", err)
	}

	mnemo := "index brass unknown lecture autumn provide royal shrimp elegant wink now zebra discover swarm act ill you bullet entire outdoor tilt usage gap multiply"
	bip39Passphrase := ""
	account := uint32(0)
	index := uint32(0)
	chainID := "dev"
	signer, err := gnoclient.SignerFromBip39(mnemo, chainID, bip39Passphrase, account, index)
	if err != nil {
		return fmt.Errorf("unable to create signer: %w", err)
	}

	// Setup Realm Handler
	// Setup webservice
	cl := gnoclient.Client{
		Signer:    signer,
		RPCClient: client,
	}

	render := service.NewWebRender(logger, &cl, md)
	webhandler := gnoweb.NewWebHandler(
		ctx, // XXX
		logger,
		render,
		&staticMeta,
	)
	mux.Handle("/r/", webhandler)

	bindaddr, err := net.ResolveTCPAddr("tcp", cfg.listener)
	if err != nil {
		return fmt.Errorf("unable to resolve listener: %q", cfg.listener)
	}

	logger.Info("Running", "listener", bindaddr.String())

	// mux := http.NewServeMux()
	server := &http.Server{
		Handler:           mux,
		Addr:              bindaddr.String(),
		ReadHeaderTimeout: 60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		logger.Error("HTTP server stopped", " error:", err)
		os.Exit(1)
	}

	return nil
}
