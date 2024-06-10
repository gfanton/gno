package emitter

import (
	"context"
	"sync"

	"github.com/gnolang/gno/contribs/gnodev/pkg/events"
)

type LocalServer struct {
	lastEvent events.Event
	cond      *sync.Cond
	index     int
}

func NewLocalServer() *LocalServer {
	return &LocalServer{
		cond: sync.NewCond(&sync.Mutex{}),
	}
}

func (m *LocalServer) Emit(evt events.Event) {
	m.cond.L.Lock()
	m.lastEvent = evt
	m.index++
	m.cond.Broadcast()
	m.cond.L.Unlock()
}

func (m *LocalServer) Recv(ctx context.Context) <-chan events.Event {
	sub := make(chan events.Event, 1)
	m.cond.L.Lock()
	index := m.index
	go func() {
		defer m.cond.L.Unlock()

		for m.lastEvent == nil || m.index == index {
			m.cond.Wait()
		}

		select {
		case <-ctx.Done():
		case sub <- m.lastEvent:
		}

		close(sub)
	}()

	return sub
}
