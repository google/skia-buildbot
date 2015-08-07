package eventbus

import (
	"sync"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/geventbus"
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

	// Optional global eventbus used to distribute events over the network.
	globalEventBus geventbus.GlobalEventBus

	// Keeps track of whether we have already subsribed to specific global event.
	globallySubscribed map[string]bool
}

// Internal struct to keep keep track of an event and it's handlers.
type topicHandler struct {
	callbacks []CallbackFn
	wg        sync.WaitGroup
}

// GlobalEvents stores a map[topic]LRUCodec of global events that should
// be available through this eventbus.
var globalEvents map[string]util.LRUCodec = map[string]util.LRUCodec{}

// globalEventsMutex protects globalEvents.
var globalEventsMutex sync.Mutex

// instanceCreated keeps track whether at least one instance of EventBus
// has been created. If so we don't allow any more subscriptions.
var instanceCreated = false

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

	// If a client tries to register an event after an EventBus has been
	// instantiated we panic, because this constitutes a programming error.
	if instanceCreated {
		panic("Trying to register a global event after an instance of EventBus has already been created.")
	}
	globalEvents[topic] = codec
}

// New returns a new instance of EventBus
func New(globalEventBus geventbus.GlobalEventBus) *EventBus {
	globalEventsMutex.Lock()
	defer globalEventsMutex.Unlock()
	instanceCreated = true

	// Make sure the global event bus does not double send what we already dispatched locally.
	if globalEventBus != nil {
		globalEventBus.DispatchSentMessages(false)
	}

	ret := &EventBus{
		globalEventBus: globalEventBus,
		handlers:       map[string]*topicHandler{},
	}
	return ret
}

// Publish publishes events for the provided topic. arg is passed
// to all functions that have subscribed to this event. The type
// and value of arg is event dependent.
func (e *EventBus) Publish(topic string, arg interface{}) {
	e.publishEvent(topic, arg, e.globalEventBus != nil)
}

func (e *EventBus) publishEvent(topic string, arg interface{}, globally bool) {
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

	if globally {
		if eventCodec, ok := globalEvents[topic]; ok {
			go func() {
				byteData, err := eventCodec.Encode(arg)
				if err != nil {
					glog.Errorf("Unable to encode event data for topic %s:  %s", topic, err)
					return
				}

				if err := e.globalEventBus.Publish(topic, byteData); err != nil {
					glog.Errorf("Unable to publish global event for topic %s:  %s", topic, err)
				}
			}()
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
	if err := e.registerGlobalSubscription(topic); err != nil {
		glog.Errorf("Unable to register for global event topic %s:  %s", topic, err)
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

func (e *EventBus) registerGlobalSubscription(topic string) error {
	if e.globalEventBus == nil {
		return nil
	}

	// This is not a global topic nothing to do here.
	eventCodec, ok := globalEvents[topic]
	if !ok {
		return nil
	}

	// We already subscribed to this global topic. Nothing to do.
	if _, ok := e.globallySubscribed[topic]; ok {
		return nil
	}

	err := e.globalEventBus.SubscribeAsync(topic, func(data []byte) {
		// deserialize the instance.
		inst, err := eventCodec.Decode(data)
		if err != nil {
			glog.Errorf("Error decoding global event for topic %s:  %s", topic, err)
			return
		}

		// Publish the global event locally.
		e.publishEvent(topic, inst, false)
	})

	return err
}
