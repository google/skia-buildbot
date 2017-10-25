package eventbus

import (
	"sync"
)

// CallbackFn defines the signature of all callback functions used for
// callbacks by the EventBus interface.
type CallbackFn func(data interface{})

// EventBus defines an interface for a generic event that
// allows to send arbitrary data on multiple channels.
type EventBus interface {
	// Publish sends the given data to all functions that have
	// registered for the given channel. Each callback function is
	// called on a separate go-routine.
	// globally indicates whether the event should distributed across machines
	// if the event bus implementation support this. It is ignored otherwise.
	// If the message cannot be sent for some reason an error will be logged.
	Publish(channel string, data interface{}, globally bool)

	// SubscribeAsync allows to register a callback function for the given
	// channel. It is assumed that the subscriber and publisher know what
	// types are sent on each channel.
	SubscribeAsync(eventType string, callback CallbackFn)
}

// MemEventBus implement the EventBus interface for an in-process event bus.
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

// New returns a new in-process event bus that can used to notify
// different components about events.
func New() EventBus {
	ret := &MemEventBus{
		handlers: map[string]*channelHandler{},
	}
	return ret
}

// Publish implements the EventBus interface.
func (e *MemEventBus) Publish(channel string, arg interface{}, globally bool) {
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

// SubscribeAsync implements the EventBus interface.
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
