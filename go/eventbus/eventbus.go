package eventbus

import "sync"

// CallbackFn defines the signature of all callback functions.
type CallbackFn func(interface{})

// EventBus is a minimal eventbus that allows to subscribe to events
// and publish events.
type EventBus struct {
	handlers map[string][]CallbackFn
	mutex    sync.Mutex
}

// New returns a new instance of EventBus
func New() *EventBus {
	return &EventBus{
		handlers: map[string][]CallbackFn{},
	}
}

// Publish publishes events for the provided topic. arg is passed
// to all functions that have subscribed to this event. The type
// and value of arg is event dependent.
func (e *EventBus) Publish(topic string, arg interface{}) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if handlers, ok := e.handlers[topic]; ok {
		for _, handler := range handlers {
			go handler(arg)
		}
	}
}

// SubscribeAsync subscribes to the given topic. When an event for topic
// is published the callback function will be called. All function calls
// are asynchronous, i.e. run in a separate goroutine.
func (e *EventBus) SubscribeAsync(topic string, callback CallbackFn) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	e.handlers[topic] = append(e.handlers[topic], callback)
}
