package eventbus

import (
	"sync"

	"go.skia.org/infra/go/util"
)

// CallbackFn defines the signature of all callback functions.
type CallbackFn func(interface{})

// EventBus is a minimal eventbus that allows to subscribe to events
// and publish events.
type EventBus struct {
	// Map of handlers keyed by topic. This is used to keep track of subscriptions.
	handlers map[string]*topicHandler

	// Used to protect handlers.
	mutex sync.Mutex
}

// Internal struct to keep keep track of an event and it's handlers.
type topicHandler struct {
	callbacks []CallbackFn
	wg        sync.WaitGroup
}

// SubTopicFilter is used to accept or reject messages from a topic for inclusion in a sub topic.
type SubTopicFilter func(eventData interface{}) bool

// subTopicEntry stores the information to create a topic by filtering a topic.
type subTopicEntry struct {
	// topic to be filtered.
	topic string

	// filter function that accepts or rejects a message from the underlying topic.
	filterFn SubTopicFilter
}

var (
	// GlobalEvents stores a map[topic]LRUCodec of global events that should
	// be available through this eventbus.
	globalEvents map[string]util.LRUCodec = map[string]util.LRUCodec{}

	// globalEventsMutex protects globalEvents.
	globalEventsMutex sync.Mutex

	// subTopics stores topics that we generate by filtering other topics.
	subTopics map[string]*subTopicEntry = map[string]*subTopicEntry{}

	// subTopicsMutex protects subTopics.
	subTopicsMutex sync.Mutex
)

// RegisterGlobalEvent registers a global event to be handled by
// instances of EventBus. A global event is identified by a topic(string)
// and a codec that can translate between go data structures and raw
// bytes slices.
// Events need to be be registered before an instance of EventBus is
// created. Ideally in the "init" function of the package that defines the
// event. This is necessary, because global events are used accross
// applications and best shared via the shared packages.
func RegisterGlobalEvent(topic string, codec util.LRUCodec) {
	globalEventsMutex.Lock()
	defer globalEventsMutex.Unlock()
	globalEvents[topic] = codec
}

// RegisterSubTopic creates an event topic that is derived from an existing topic by
// applying a filter. In the background it subscribes to 'topic'. If it receives an
// event for topic it invokes the filter function. If the filter function returns true
// it will emit an event for the sub topic.
func RegisterSubTopic(topic, subTopic string, filterFn SubTopicFilter) {
	subTopicsMutex.Lock()
	defer subTopicsMutex.Unlock()
	subTopics[subTopic] = &subTopicEntry{
		topic:    topic,
		filterFn: filterFn,
	}
}

// New returns a new instance of EventBus
func New() *EventBus {
	globalEventsMutex.Lock()
	defer globalEventsMutex.Unlock()

	ret := &EventBus{
		handlers: map[string]*topicHandler{},
	}
	return ret
}

// Publish publishes events for the provided topic. arg is passed
// to all functions that have subscribed to this event. The type
// and value of arg is event dependent.
func (e *EventBus) Publish(topic string, arg interface{}) {
	e.publishEvent(topic, arg)
}

func (e *EventBus) publishEvent(topic string, arg interface{}) {
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

	if !e.subscribeToSubTopic(topic, callback) {
		e.subscribeWithLock(topic, callback)
	}
}

// subscribeWithLocks registers the given callback for the given topic. It assumes
// that the subscription process has been locked with e.mutex.
func (e *EventBus) subscribeWithLock(topic string, callback CallbackFn) {
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

// subscribeToSubTopic returns true if the given 'subTopic' is indeed a registered sub topic.
// In that case it will register for the underlying topic to filter events. This results in
// a recursive call to SubscribeAsync.
func (e *EventBus) subscribeToSubTopic(subTopic string, callback CallbackFn) bool {
	subTopicsMutex.Lock()
	entry, ok := subTopics[subTopic]
	subTopicsMutex.Unlock()
	if ok {
		e.subscribeWithLock(entry.topic, func(data interface{}) {
			if entry.filterFn(data) {
				callback(data)
			}
		})
	}
	return ok
}
