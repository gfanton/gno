package emitter

import (
	"sync"

	"github.com/gnolang/gno/contribs/gnodev/pkg/events"
)

type Emitter interface {
	Emit(evt events.Event)
}

type combineServer struct {
	emitters []Emitter
}

func (e combineServer) Emit(evt events.Event) {
	var wg sync.WaitGroup
	wg.Add(len(e.emitters))

	for _, e := range e.emitters {
		go func(e Emitter) {
			// Defer here to avoid deadlock on panic recover
			defer wg.Done()

			e.Emit(evt)
		}(e)
	}

	wg.Wait()
}

func Combine(emitters ...Emitter) Emitter {
	return &combineServer{
		emitters: emitters,
	}
}
