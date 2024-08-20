package gnoweb

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"regexp"

	"github.com/a-h/templ"
	"github.com/gnolang/gno/contribs/gnoweb2/components"
	"github.com/gnolang/gno/contribs/gnoweb2/pkg/service"
)

type StaticMetadata struct {
	AssetsPath string
}

type WebHandler struct {
	logger *slog.Logger
	static *StaticMetadata
	render service.Render
	ctx    context.Context
}

func NewWebHandler(ctx context.Context, logger *slog.Logger, render service.Render, meta *StaticMetadata) *WebHandler {
	return &WebHandler{
		render: render,
		ctx:    ctx,
		logger: logger.WithGroup("web"),
		static: meta,
	}
}

func (h *WebHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	h.logger.Info("receving request", "path", r.URL.Path)
	switch r.Method {
	case http.MethodGet:
		h.Get(w, r)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// reRealName match a a realm path
// - match[1]: path of the realm
// - match[2]: args of the realm
var reRealmName = regexp.MustCompile(`(?mU)(/r/[a-z][a-z0-9_]*(?:/[a-z][a-z0-9_]*?))+(?:\:(.*?))??`)

func (h *WebHandler) Get(w http.ResponseWriter, r *http.Request) {
	matchs := reRealmName.FindStringSubmatch(r.URL.Path)

	var realm templ.Component
	if len(matchs) > 0 {
		path, args := matchs[1], matchs[2]
		realm = templ.ComponentFunc(func(ctx context.Context, w io.Writer) error {
			if err := h.render.Render(w, path, args); err != nil {
				h.logger.Error("unable to render", "err", err)
				components.NotFoundComponent("realm not found").Render(h.ctx, w)
			}

			return nil
		})
	}

	var metadata components.PageMetadata
	metadata.Title = "MyRealm"

	components.IndexView(metadata, realm).Render(h.ctx, w)
}
