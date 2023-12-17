package events

import (
	"fmt"
	"log"
	"reflect"
	"sync"
	"time"
)

// Returns a synchronous event emitter.
func Subscribe(evsw EventSwitch, listenerID string) <-chan Event {
	ch := make(chan Event, 0) // synchronous
	return SubscribeOn(evsw, listenerID, ch)
}

// Like Subscribe, but lets the caller construct a channel.  If the capacity of
// the provided channel is 0, it will be called synchronously; otherwise, it
// will drop when the capacity is reached and a select doesn't immediately
// send.
func SubscribeOn(evsw EventSwitch, listenerID string, ch chan Event) <-chan Event {
	return SubscribeFilteredOn(evsw, listenerID, nil, ch)
}

func SubscribeToEvent(evsw EventSwitch, listenerID string, protoevent Event) <-chan Event {
	ch := make(chan Event, 0) // synchronous
	return SubscribeToEventOn(evsw, listenerID, protoevent, ch)
}

func SubscribeToEventOn(evsw EventSwitch, listenerID string, protoevent Event, ch chan Event) <-chan Event {
	rt := reflect.TypeOf(protoevent)
	return SubscribeFilteredOn(evsw, listenerID, func(event Event) bool {
		return reflect.TypeOf(event) == rt
	}, ch)
}

type EventFilter func(Event) bool

func SubscribeFiltered(evsw EventSwitch, listenerID string, filter EventFilter) <-chan Event {
	ch := make(chan Event, 0)
	return SubscribeFilteredOn(evsw, listenerID, filter, ch)
}

var (
	ms    = make(map[string]int)
	slock = sync.Mutex{}
)

func printMap(ms map[string]int) {
	fmt.Printf("--- \n")
	for k, v := range ms {
		fmt.Printf(" [%s: %d]", k, v)
	}
	fmt.Printf("--- \n")
}

func SubscribeFilteredOn(evsw EventSwitch, listenerID string, filter EventFilter, ch chan Event) <-chan Event {
	var onceClose sync.Once
	evsw.AddListener(listenerID, func(event Event) {
		if filter != nil && !filter(event) {
			return // filter
		}

		ms[listenerID]++
		printMap(ms)
		defer func() {
			ms[listenerID]--
			if ms[listenerID] == 0 {
				delete(ms, listenerID)
				return
			}
			printMap(ms)
		}()

		// NOTE: This callback must not block for performance.
		if cap(ch) == 0 {
			timeout := 10 * time.Second
		LOOP:
			for {
				select { // sync
				case ch <- event:
					break LOOP
				case <-evsw.Quit():
					evsw.RemoveListener(listenerID) // TODO log
					onceClose.Do(func() { close(ch) })
					break LOOP
				case <-time.After(timeout):
					// After a minute, print a message for debugging.
					log.Printf("[WARN] EventSwitch subscriber %v blocked on %v for %v", listenerID, event, timeout)
					// Exponentially back off warning messages.
					timeout *= 2
				}
			}
		} else {
			select {
			case ch <- event:
			default: // async
				evsw.RemoveListener(listenerID) // TODO log
				onceClose.Do(func() { close(ch) })
			}
		}
	})
	return ch
}
