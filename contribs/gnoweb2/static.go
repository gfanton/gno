package gnoweb

import (
	"embed"
	_ "embed"
	"net/http"
)

//go:embed assets
var assets embed.FS

func AssetHandler() http.Handler {
	return http.FileServer(http.FS(assets))
}
