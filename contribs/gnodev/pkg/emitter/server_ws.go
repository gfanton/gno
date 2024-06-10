package emitter

import (
	"context"
	"log/slog"
	"net/http"
	"sync"

	"github.com/gnolang/gno/contribs/gnodev/pkg/events"
	"github.com/gnolang/gno/gno.land/pkg/sdk/vm"
	"github.com/gorilla/websocket"
)

type WSServer struct {
	logger    *slog.Logger
	upgrader  websocket.Upgrader
	clients   map[*websocket.Conn]struct{}
	muClients sync.RWMutex
}

func NewWSServer(logger *slog.Logger) *WSServer {
	return &WSServer{
		logger:  logger,
		clients: make(map[*websocket.Conn]struct{}),
		upgrader: websocket.Upgrader{
			CheckOrigin: func(r *http.Request) bool {
				return true // XXX: adjust this
			},
		},
	}
}

// ws handler
func (s *WSServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := s.upgrader.Upgrade(w, r, nil)
	if err != nil {
		s.logger.Error("unable to upgrade connection", "remote", r.RemoteAddr, "error", err)
		return
	}
	defer conn.Close()

	s.muClients.Lock()
	s.clients[conn] = struct{}{}
	s.muClients.Unlock()

	for {
		_, _, err := conn.ReadMessage()
		if err != nil {
			s.muClients.Lock()
			delete(s.clients, conn)
			s.muClients.Unlock()
			break
		}
	}
}

func (s *WSServer) Emit(evt events.Event) {
	go s.emit(evt)
}

type eventJSON struct {
	Type events.Type `json:"type"`
	Data any         `json:"data"`
}

func (s *WSServer) emit(evt events.Event) {
	s.muClients.Lock()
	defer s.muClients.Unlock()

	jsonEvt := eventJSON{evt.Type(), evt}

	s.logevent(evt)

	for conn := range s.clients {
		err := conn.WriteJSON(jsonEvt)
		if err != nil {
			s.logger.Error("write json event", "error", err)
			conn.Close()
			delete(s.clients, conn)
		}
	}
}

func (s *WSServer) logevent(evt events.Event) {
	var attrs []slog.Attr
	attrs = append(attrs, slog.String("type", string(evt.Type())))
	switch evt := evt.(type) {
	case events.TxResult:
		for _, msg := range evt.Tx.Msgs {
			switch msg := msg.(type) {
			case vm.MsgCall:
				attrs = append(attrs, slog.Any("msg", map[string]any{
					"Type":    "MsgCall",
					"PkgPath": msg.PkgPath,
					"Func":    msg.Func,
					"Args":    msg.Args,
				}))
			case vm.MsgRun:
				attrs = append(attrs, slog.Any("msg", map[string]any{
					"Type":   "MsgRun",
					"Pkg":    msg.Package,
					"Caller": msg.Caller,
					"Coins":  msg.Send,
				}))
			case vm.MsgAddPackage:
				attrs = append(attrs, slog.Any("msg", map[string]any{
					"Type":    "MsgAddPackage",
					"Pkg":     msg.Package,
					"Creator": msg.Creator,
					"Deposit": msg.Deposit,
				}))
			default:
			}
		}

	case events.Reload:
	case events.Reset:
	}

	s.logger.LogAttrs(context.Background(), slog.LevelInfo, "sending event", attrs...)
}

func (s *WSServer) conns() []*websocket.Conn {
	s.muClients.RLock()
	conns := make([]*websocket.Conn, 0, len(s.clients))
	for conn := range s.clients {
		conns = append(conns, conn)
	}
	s.muClients.RUnlock()

	return conns
}
