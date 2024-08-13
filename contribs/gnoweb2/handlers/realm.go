package handlers

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/gnolang/gno/contribs/gnoweb2/components"
	"github.com/gnolang/gno/gno.land/pkg/gnoclient"
)

type CountService interface {
	Increment(ctx context.Context) (counts int64, err error)
	Get(ctx context.Context) (counts int64, err error)
}

func New(log *slog.Logger, cl *gnoclient.Client) *DefaultHandler {
	return &DefaultHandler{
		Log:          log,
		CountService: cs,
	}
}

type DefaultHandler struct {
	Log          *slog.Logger
	CountService CountService
}

func (h *DefaultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodPost {
		h.Post(w, r)
		return
	}
	h.Get(w, r)
}

func (h *DefaultHandler) Get(w http.ResponseWriter, r *http.Request) {
	var props ViewProps
	var err error
	props.Counts, err = h.CountService.Get(context.Background())
	if err != nil {
		h.Log.Error("failed to get counts", slog.Any("error", err))
		http.Error(w, "failed to get counts", http.StatusInternalServerError)
		return
	}
	h.View(w, r, props)
}

func (h *DefaultHandler) Post(w http.ResponseWriter, r *http.Request) {
	r.ParseForm()

	// Decide the action to take based on the button that was pressed.
	counts, err := h.CountService.Increment(r.Context())
	if err != nil {
		h.Log.Error("failed to increment", slog.Any("error", err))
		http.Error(w, "failed to increment", http.StatusInternalServerError)
		return
	}

	// Display the view.
	h.View(w, r, ViewProps{
		Counts: counts,
	})
}

type ViewProps struct {
	Counts int64
}

func (h *DefaultHandler) View(w http.ResponseWriter, r *http.Request, props ViewProps) {
	components.Page(props.Counts).Render(r.Context(), w)
}
