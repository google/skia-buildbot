package eventbus

import (
	"sync"
)

// TODO remove we probably won't need this.
// type Event struct {
// 	ID        string      `json:"id"`
// 	EventType string      `json:"eventType"`
// 	Data      interface{} `json:"data"`
// }

// CallbackFn defines the signature of all callback functions.
type CallbackFn func(data interface{})

type EventBus interface {
	Publish(eventType string, data interface{})
	SubscribeAsync(eventType string, callback CallbackFn)
}

// EventBus is a minimal eventbus that allows to subscribe to events
// and publish events.
type MemEventBus struct {
	// Map of handlers keyed by channel. This is used to keep track of subscriptions.
	handlers map[string]*channelHandler

	// Used to protect handlers.
	mutex sync.Mutex
}

// Internal struct to keep keep track of an event and it's handlers.
type channelHandler struct {
	callbacks []CallbackFn
	wg        sync.WaitGroup
}

// New returns a new instance of EventBus
func New() EventBus {
	ret := &MemEventBus{
		handlers: map[string]*channelHandler{},
	}
	return ret
}

// Publish publishes events for the provided channel. arg is passed
// to all functions that have subscribed to this event. The type
// and value of arg is event dependent.
func (e *MemEventBus) Publish(channel string, arg interface{}) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if th, ok := e.handlers[channel]; ok {
		for _, callback := range th.callbacks {
			th.wg.Add(1)
			go func(callback CallbackFn) {
				defer th.wg.Done()
				callback(arg)
			}(callback)
		}
	}
}

// Subscribe subscribes to the given channel. When an event for channel
// is published the callback function will be called. All function calls
// are asynchronous, i.e. run in a separate goroutine.
func (e *MemEventBus) SubscribeAsync(channel string, callback CallbackFn) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if th, ok := e.handlers[channel]; ok {
		th.callbacks = append(th.callbacks, callback)
	} else {
		e.handlers[channel] = &channelHandler{callbacks: []CallbackFn{callback}}
	}
}

// Wait will block until the goroutines for a specific channel have finished.
func (e *MemEventBus) Wait(channel string) {
	// Block briefly to find the handler struct for the channel.
	e.mutex.Lock()
	th, ok := e.handlers[channel]
	e.mutex.Unlock()
	if ok {
		th.wg.Wait()
	}
}
