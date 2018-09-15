package eventbus

import (
	"regexp"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/sklog"
)

const (
	// maxConcurrentPublishers is the maximum number of go-routines that can publish events concurrently.
	maxConcurrentPublishers = 1000

	// SYN_STORAGE_EVENT is the event type for synthetic storage events that are sent via the
	// PublishStorageEvent function.
	SYN_STORAGE_EVENT = "eventbus:synthetic-storage-event"

	// storageEventPrefix is the prefix of all storage event types to
	// distinguish them from user defined event types.
	storageEventPrefix = "--storage-event-"

	invalidObjectPrefix = ".."
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

	// PublishStorageEvent publishes a synthetic storage event that is handled by
	// registered storage event handlers.
	PublishStorageEvent(bucketName, objectName string)
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

	// concurrentPub is used the limit the number of go-routines that can concurrently
	// publish events. Since each Publish call can spin up multiple go-routines we avoid
	// creating too many. In most cases the maximum will never be reached.
	concurrentPub chan bool

	// Used to protect handlers.
	mutex sync.Mutex

	notificationsMap *NotificationsMap
}

// Internal struct to keep keep track of an event and it's handlers.
type channelHandler struct {
	callbacks []CallbackFn
}

// New returns a new in-process event bus that can used to notify
// different components about events.
func New() EventBus {
	ret := &MemEventBus{
		handlers:         map[string]*channelHandler{},
		concurrentPub:    make(chan bool, maxConcurrentPublishers),
		notificationsMap: NewNotificationsMap(),
	}
	return ret
}

// Publish implements the EventBus interface.
func (e *MemEventBus) Publish(channel string, arg interface{}, globally bool) {
	// If this is a synthethic storage event then reframe it an actual storage event.
	if channel == SYN_STORAGE_EVENT {
		evt := arg.(*StorageEvent)
		eventTypes := e.notificationsMap.Matches(evt.BucketID, evt.ObjectID)
		for _, eventType := range eventTypes {
			e.Publish(eventType, arg, true)
		}
		return
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()
	if th, ok := e.handlers[channel]; ok {
		for _, callback := range th.callbacks {
			e.concurrentPub <- true
			go func(callback CallbackFn) {
				defer func() { <-e.concurrentPub }()
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
	notificationsID := e.notificationsMap.GetNotificationID(bucketName, objectPrefix)
	return e.notificationsMap.Add(notificationsID, objectRegEx), nil
}

// PublishStorageEvent implements the EventBus interface.
func (e *MemEventBus) PublishStorageEvent(bucketName, objectName string) {
	evtData := NewStorageEvent(storage.ObjectFinalizeEvent, bucketName, objectName)
	e.Publish(SYN_STORAGE_EVENT, evtData, true)
}

func NewStorageEvent(storageEventType, bucketID, objectID string) *StorageEvent {
	return &StorageEvent{
		EventType: storageEventType,
		BucketID:  bucketID,
		ObjectID:  objectID,
	}
}

type NotificationsMap struct {
	notifications map[string]map[string]*regexp.Regexp
}

func NewNotificationsMap() *NotificationsMap {
	return &NotificationsMap{notifications: map[string]map[string]*regexp.Regexp{}}
}

func (n *NotificationsMap) GetNotificationID(bucketName, objectPrefix string) string {
	return bucketName + "/" + strings.TrimLeft(objectPrefix, "/")
}

func (n *NotificationsMap) Add(notifyID string, objectRegEx *regexp.Regexp) string {
	// If no regex was provided we add a single entry.
	regexStr := ""
	if objectRegEx != nil {
		regexStr = objectRegEx.String()
	}
	if _, ok := n.notifications[notifyID]; !ok {
		n.notifications[notifyID] = map[string]*regexp.Regexp{}
	}
	n.notifications[notifyID][regexStr] = objectRegEx
	return getEventType(notifyID, objectRegEx)
}

func getEventType(notificationID string, regEx *regexp.Regexp) string {
	regexStr := ""
	if regEx != nil {
		regexStr = regEx.String()
	}
	return storageEventPrefix + notificationID + "/" + regexStr
}

func splitNotificationID(notificationID string) (string, string) {
	parts := strings.SplitN(notificationID, "/", 2)
	if len(parts) != 2 {
		sklog.Errorf("Logic error. Received notificationID '%s' without a '/'", notificationID)
		return "", invalidObjectPrefix
	}
	return parts[0], parts[1]
}

func (n *NotificationsMap) MatchesByID(notificationID, objectID string) []string {
	// Find the notification ID if it's not registered no event types are returned.
	regexes, ok := n.notifications[notificationID]
	if !ok {
		return []string{}
	}

	// Check if the given objectID matches the found objectPrefix.
	_, objectPrefix := splitNotificationID(notificationID)
	if !strings.HasPrefix(objectID, objectPrefix) {
		return []string{}
	}

	return getEventTypesFromRegexps(notificationID, objectID, regexes)
}

func getEventTypesFromRegexps(notificationID, objectID string, regexes map[string]*regexp.Regexp) []string {
	// Check the objectID against the regular expressions.
	ret := make([]string, 0, len(regexes))
	for id, oneRegEx := range regexes {
		if id == "" || oneRegEx.Match([]byte(objectID)) {
			ret = append(ret, getEventType(notificationID, oneRegEx))
		}
	}
	return ret
}

func (n *NotificationsMap) Matches(evtBucketID, evtObjectID string) []string {
	// Iterate over all notifications and check if they match.
	ret := []string{}
	for notifyID, regexes := range n.notifications {
		bucketID, objectPrefix := splitNotificationID(notifyID)
		if evtBucketID == bucketID && strings.HasPrefix(evtObjectID, objectPrefix) {
			ret = append(ret, getEventTypesFromRegexps(notifyID, evtObjectID, regexes)...)
		}
	}
	return ret
}
