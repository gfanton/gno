package main

import (
	"log/slog"
	"net/http"

	gnodev "github.com/gnolang/gno/contribs/gnodev/pkg/dev"
	"github.com/gnolang/gno/gno.land/pkg/gnoweb"
)

// setupGnowebServer initializes and starts the Gnoweb server.
func setupGnoWebServer(logger *slog.Logger, cfg *devCfg, dnode *gnodev.Node) http.Handler {
	webConfig := gnoweb.NewDefaultConfig()
	webConfig.HelpChainID = cfg.chainId
	webConfig.RemoteAddr = dnode.GetRemoteAddress()

	// If `HelpRemote` is empty default it to `RemoteAddr`
	webConfig.HelpRemote = cfg.webRemoteHelperAddr
	if webConfig.HelpRemote == "" {
		webConfig.HelpRemote = dnode.GetRemoteAddress()
	}

	app := gnoweb.MakeApp(logger, webConfig)
	return app.Router
}
