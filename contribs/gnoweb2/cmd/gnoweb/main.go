package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"

	"github.com/gnolang/gno/contribs/gnoweb2/pkg/gnoweb"
	"github.com/gnolang/gno/gno.land/pkg/log"
	"github.com/gnolang/gno/tm2/pkg/commands"
	"go.uber.org/zap/zapcore"
)

type webCfg struct {
	listener string
	loglevel string
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

}

func execWeb(cfg *webCfg, args []string, io commands.IO) (err error) {
	zapLogger := log.NewZapConsoleLogger(os.Stdout, zapcore.DebugLevel)
	defer zapLogger.Sync()

	// Setup logger
	logger := log.ZapLoggerToSlog(zapLogger)

	// Setup webservice
	scfg := gnoweb.NewDefaultConfig()
	scfg.Logger = logger
	svc := gnoweb.NewService(scfg)


	// Setup Handler
	handler := gnoweb.DefaultHandler{
		Logger:     logger,
		WebService: svc,
	}

	bindaddr, err := net.ResolveTCPAddr("tcp", cfg.listener)
	if err != nil {
		return fmt.Errorf("unable to resolve listener: %q", cfg.listener)
	}

	logger.Info("Running", "listener", bindaddr.String())

	// mux := http.NewServeMux()
	server := &http.Server{
		Handler:           &handler,
		Addr:              bindaddr.String(),
		ReadHeaderTimeout: 60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		logger.Error("HTTP server stopped", " error:", err)
		os.Exit(1)
	}

	return nil
}
