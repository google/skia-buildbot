package eventbus

import (
	"regexp"
	"sync"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/sklog"
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

	// RegisterStorageEvents registers to receive storage events for the given
	// bucket.
	//  bucketName - global name of the target bucket
	//  objectPrefix - filter objects (server side) that have this prefix.
	//  objectRegEx - only include objects where the name matches this regular
	//                expression (can be nil). Client side filtering.
	//  client - Google storage client that has permission to create a
	//           pubsub based event subscription for the given bucket.
	//
	// Returns: event type to use in the SubscribeAsync call to receive events
	//          for this combination of (bucketName, objectPrefix, objectRegEx).
	//
	// Note: objectPrefix filters events on the server side, i.e. they never reach
	//       cause a PubSub event to be fired. objectRegEx filter events on the
	//       client side by matching against an objects name,
	//       e.g. ".*\.json$" would only include JSON files.
	//
	// Currently this is only implemented by the gevent package.
	RegisterStorageEvents(bucketName string, objectPrefix string, objectRegEx *regexp.Regexp, client *storage.Client) (string, error)
}

// StorageEvent is the type of object that is published by GCS storage events.
// Note: These events need to be registered with RegisterStorageEvents.
type StorageEvent struct {
	// EventType is the event type supplied by GCS.
	// See https://cloud.google.com/storage/docs/pubsub-notifications#events
	EventType string

	// BucketID is the name of the bucket that create the event.
	BucketID string

	// ObjectID is the name/path of the object that triggered the event.
	ObjectID string
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

// RegisterStorageEvent implements the EventBus interface.
func (e *MemEventBus) RegisterStorageEvents(bucketName string, objectPrefix string, objectRegEx *regexp.Regexp, client *storage.Client) (string, error) {
	return "", sklog.FmtErrorf("Function RegisterStorageEvents not implemented by MemEventBus - see gevent package instead")
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
