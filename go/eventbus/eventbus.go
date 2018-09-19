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

	// invalidObjectPrefix is used to as a sentinel value in the case of an invalid
	// notification id (see below) to prevent fake vents from being fired.
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
	RegisterStorageEvents(bucketName string, objectPrefix string, objectRegEx *regexp.Regexp, client *storage.Client) (string, error)

	// PublishStorageEvent publishes a synthetic storage event that is handled by
	// registered storage event handlers. All storage events are global.
	PublishStorageEvent(evtData *StorageEvent)
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

	// The generation number of the object that was overwritten by the object
	// that this notification pertains to. This attribute only appears in
	// OBJECT_FINALIZE events in the case of an overwrite.
	OverwroteGeneration string

	// MD5 is the MD5 hash of the object.
	MD5 string

	// TimeStamp is the time of the last update in Unix time (seconds since the epoch).
	TimeStamp int64
}

// NewStorageEvent is a convenience method to create a new StorageEvent. Currently all
// instances have storage.ObjectFinalizeEvent as EventType. This indicates a new object
// being created.
func NewStorageEvent(bucketID, objectID string, lastUpdated int64, md5 string) *StorageEvent {
	return &StorageEvent{
		EventType: storage.ObjectFinalizeEvent,
		BucketID:  bucketID,
		ObjectID:  objectID,
		TimeStamp: lastUpdated,
		MD5:       md5,
	}
}

// MemEventBus implement the EventBus interface for an in-process event bus.
type MemEventBus struct {
	// Map of handlers keyed by channel. This is used to keep track of subscriptions.
	handlers map[string]channelHandler

	// concurrentPub is used the limit the number of go-routines that can concurrently
	// publish events. Since each Publish call can spin up multiple go-routines we avoid
	// creating too many. In most cases the maximum will never be reached.
	concurrentPub chan bool

	// Used to protect handlers.
	mutex sync.Mutex

	// storageNotifications keep track of storage notifications. Mainly used for
	// testing in with the in-memory eventbus.
	storageNotifications *NotificationsMap
}

// Internal type to keep keep track of an event and it's handlers.
type channelHandler []CallbackFn

// New returns a new in-process event bus that can used to notify
// different components about events.
func New() EventBus {
	ret := &MemEventBus{
		handlers:             map[string]channelHandler{},
		concurrentPub:        make(chan bool, maxConcurrentPublishers),
		storageNotifications: NewNotificationsMap(),
	}
	return ret
}

// Publish implements the EventBus interface.
func (e *MemEventBus) Publish(channel string, arg interface{}, globally bool) {
	// If this is a synthethic storage event then reframe it an actual storage event.
	if channel == SYN_STORAGE_EVENT {
		evt := arg.(*StorageEvent)
		eventTypes := e.storageNotifications.Matches(evt.BucketID, evt.ObjectID)
		for _, eventType := range eventTypes {
			e.Publish(eventType, arg, true)
		}
		return
	}

	e.mutex.Lock()
	defer e.mutex.Unlock()
	if callbacks, ok := e.handlers[channel]; ok {
		for _, callback := range callbacks {
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
	if callbacks, ok := e.handlers[channel]; ok {
		e.handlers[channel] = append(callbacks, callback)
	} else {
		e.handlers[channel] = []CallbackFn{callback}
	}
}

// RegisterStorageEvent implements the EventBus interface.
func (e *MemEventBus) RegisterStorageEvents(bucketName string, objectPrefix string, objectRegEx *regexp.Regexp, client *storage.Client) (string, error) {
	notificationsID := e.storageNotifications.GetNotificationID(bucketName, objectPrefix)
	return e.storageNotifications.Add(notificationsID, objectRegEx), nil
}

// PublishStorageEvent implements the EventBus interface.
func (e *MemEventBus) PublishStorageEvent(evtData *StorageEvent) {
	e.Publish(SYN_STORAGE_EVENT, evtData, true)
}

// NotificationsMap is a helper type that keep tracks of storage events. It
// assumes that storage events mainly consist of buckets and objects and
// related meta data. It is used by MemEventBus and the PubSub based eventbus
// implemented in the gevent package.
type NotificationsMap struct {
	notifications map[string]map[string]*regexp.Regexp
}

// NewNotifications creates a new instance of NotificationsMap
func NewNotificationsMap() *NotificationsMap {
	return &NotificationsMap{notifications: map[string]map[string]*regexp.Regexp{}}
}

// GetNotificationID returns a string that is a combination of a bucket and an object prefix
// representing a server-side subscription to storage events.
func (n *NotificationsMap) GetNotificationID(bucketName, objectPrefix string) string {
	return bucketName + "/" + strings.TrimLeft(objectPrefix, "/")
}

// Add adds a notification to the map it consists of a notification id (created via GetNotificationID)
// and regular expression. The regex can be nil. If not nil, it will be used for client side
// filtering of object IDs that are delivered by events.
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

// MatchesByID checks if the given objectID matches the given notificationID and
// the regular expression associated with it. This requires that the notificationID
// and the regex has been added previously via the Add method.
// It returns a list of event types for which this is a map.
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

// Matches checks whether the given bucketID and objectID are in the recorded
// list of notifications and the regular expressions associated with them.
// It returns the event types that match the found events.
func (n *NotificationsMap) Matches(bucketID, objectID string) []string {
	// Iterate over all notifications and check if they match.
	ret := []string{}
	for notifyID, regexes := range n.notifications {
		notifyBucketID, objectPrefix := splitNotificationID(notifyID)
		if bucketID == notifyBucketID && strings.HasPrefix(objectID, objectPrefix) {
			ret = append(ret, getEventTypesFromRegexps(notifyID, objectID, regexes)...)
		}
	}
	return ret
}

// getEventType returns a unique event type for the given pair of notificationID and
// regular expression. It can be used to subscribe to a storage event after subscribing to it.
func getEventType(notificationID string, regEx *regexp.Regexp) string {
	regexStr := ""
	if regEx != nil {
		regexStr = regEx.String()
	}
	return storageEventPrefix + notificationID + "/" + regexStr
}

// splitNotificationID is the inverse operation of GetNotificationID in that it returns the
// bucket and object prefix of a notification subscription.
func splitNotificationID(notificationID string) (string, string) {
	parts := strings.SplitN(notificationID, "/", 2)
	if len(parts) != 2 {
		sklog.Errorf("Logic error. Received notificationID '%s' without a '/'", notificationID)
		return "", invalidObjectPrefix
	}
	return parts[0], parts[1]
}

// getEventTypesFromRegexps check whether the given objectID matches the regular expressions
// and generates event types. This assumes that the objectID has already been confirmed as
// matching the prefix encoded in the notificationID.
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
