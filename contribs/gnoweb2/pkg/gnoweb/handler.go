package gnoweb

import (
	"context"
	"log/slog"
	"net/http"
)

type DefaultHandler struct {
	Logger     *slog.Logger
	WebService *WebServiceRender
}

func (h *DefaultHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	default:
		h.Get(w, r)
	}
}

func (h *DefaultHandler) Get(w http.ResponseWriter, r *http.Request) {
	Page("Eddy").Render(context.Background(), w)
}

// func (h *DefaultHandler) View(w http.ResponseWriter, r *http.Request, props ViewProps) {
// 	components.Page(props.Counts.Global, props.Counts.Session).Render(r.Context(), w)
// }

// func (h *DefaultHandler) Post(w http.ResponseWriter, r *http.Request) {
// 	r.ParseForm()

// 	// Decide the action to take based on the button that was pressed.
// 	var it services.IncrementType
// 	if r.Form.Has("global") {
// 		it = services.IncrementTypeGlobal
// 	}
// 	if r.Form.Has("session") {
// 		it = services.IncrementTypeSession
// 	}

// 	counts, err := h.CountService.Increment(r.Context(), it, session.ID(r))
// 	if err != nil {
// 		h.Log.Error("failed to increment", slog.Any("error", err))
// 		http.Error(w, "failed to increment", http.StatusInternalServerError)
// 		return
// 	}

// 	// Display the view.
// 	h.View(w, r, ViewProps{
// 		Counts: counts,
// 	})
