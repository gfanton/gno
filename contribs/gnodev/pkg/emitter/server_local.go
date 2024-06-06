package emitter

import "github.com/gnolang/gno/contribs/gnodev/pkg/events"

type LocalServer struct {
	sub chan events.Event
}

func NewLocalServer() *LocalServer {
	return &LocalServer{
		sub: make(chan events.Event, 16),
	}
}

func (m *LocalServer) Emit(evt events.Event) {
	m.sub <- evt
}

func (m *LocalServer) Sub() <-chan events.Event {
	return m.sub
}
