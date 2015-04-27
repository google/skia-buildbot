package eventbus

import "sync"

// CallbackFn defines the signature of all callback functions.
type CallbackFn func(interface{})

// EventBus is a minimal eventbus that allows to subscribe to events
// and publish events.
type EventBus struct {
	handlers map[string]*topicHandler
	mutex    sync.Mutex
}

type topicHandler struct {
	callbacks []CallbackFn
	wg        sync.WaitGroup
}

// New returns a new instance of EventBus
func New() *EventBus {
	return &EventBus{
		handlers: map[string]*topicHandler{},
	}
}

// Publish publishes events for the provided topic. arg is passed
// to all functions that have subscribed to this event. The type
// and value of arg is event dependent.
func (e *EventBus) Publish(topic string, arg interface{}) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if th, ok := e.handlers[topic]; ok {
		for _, callback := range th.callbacks {
			th.wg.Add(1)
			go func(callback CallbackFn) {
				defer th.wg.Done()
				callback(arg)
			}(callback)
		}
	}
}

// SubscribeAsync subscribes to the given topic. When an event for topic
// is published the callback function will be called. All function calls
// are asynchronous, i.e. run in a separate goroutine.
func (e *EventBus) SubscribeAsync(topic string, callback CallbackFn) {
	e.mutex.Lock()
	defer e.mutex.Unlock()
	if th, ok := e.handlers[topic]; ok {
		th.callbacks = append(th.callbacks, callback)
	} else {
		e.handlers[topic] = &topicHandler{callbacks: []CallbackFn{callback}}
	}
}

// Wait will block until the goroutines for a specific topic have finished.
func (e *EventBus) Wait(topic string) {
	// Block briefly to find the handler struct for the topic.
	e.mutex.Lock()
	th, ok := e.handlers[topic]
	e.mutex.Unlock()
	if ok {
		th.wg.Wait()
	}
}
