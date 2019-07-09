package eventbus

import (
	"regexp"
	"strings"
	"sync"

	"cloud.google.com/go/storage"
	"go.skia.org/infra/go/sklog"
)

const (
	// maxConcurrentPublishers is the maximum number of go-routines that can publish
	// events concurrently.
	maxConcurrentPublishers = 500

	// SYN_STORAGE_EVENT is the event type for synthetic storage events that are sent via the
	// PublishStorageEvent function.
	SYN_STORAGE_EVENT = "eventbus:synthetic-storage-event"

	// storageEventPrefix is the prefix of all storage channel IDs to
	// distinguish them from user defined channels.
	storageChannelIDPrefix = "--storage-channel-"

	// invalidObjectPrefix is used to as a sentinel value in the case of an invalid
	// notification id (see below) to prevent fake events from being fired.
	// '..' was chosen because it is actually illegal in GCS to name an object that so it will
	// never occur in a result coming from GCS.
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
	Publish(channelID string, data interface{}, globally bool)

	// SubscribeAsync allows to register a callback function for the given
	// channel. It is assumed that the subscriber and publisher know what
	// types are sent on each channel.
	SubscribeAsync(channelID string, callback CallbackFn)

	// RegisterStorageEvents registers to receive storage events for the given
	// bucket.
	//  bucketName - global name of the target bucket
	//  objectPrefix - filter objects (server side) that have this prefix.
	//  objectRegEx - only include objects where the name matches this regular
	//                expression (can be nil). Client side filtering.
	//  client - Google storage client that has permission to create a
	//           pubsub based event subscription for the given bucket.
	//
	// Returns: channel ID to use in the SubscribeAsync call to receive events
	//          for this combination of (bucketName, objectPrefix, objectRegEx), e.g.
	//
	//    chanID := RegisterStorageEvents("bucket-name", "tests", regexp.MustCompile(`\.json$`))
	//    eventBus.SubscribeAsync(chanID, func(data interface{}) {
	//       storageEvtData := data.(*eventbus.StorageEvent)
	//
	//        ... handle the storage event ...
	//    })
	//
	//
	// Note: objectPrefix filters events on the server side, i.e. they never reach
	//       cause a PubSub event to be fired. objectRegEx filter events on the
	//       client side by matching against an objects name,
	//       e.g. ".*\.json$" would only include JSON files.
	//       Currently it is implied that the GCS event type is always
	//       storage.ObjectFinalizeEvent which indicates that an object was created.
	//
	RegisterStorageEvents(bucketName string, objectPrefix string, objectRegEx *regexp.Regexp, client *storage.Client) (string, error)

	// PublishStorageEvent publishes a synthetic storage event that is handled by
	// registered storage event handlers. All storage events are global.
	PublishStorageEvent(evtData *StorageEvent)
}

// StorageEvent is the type of object that is published by GCS storage events.
// Note: These events need to be registered with RegisterStorageEvents.
type StorageEvent struct {
	// GCSEventType is the event type supplied by GCS.
	// See https://cloud.google.com/storage/docs/pubsub-notifications#events
	GCSEventType string

	// BucketID is the name of the bucket that create the event.
	BucketID string

	// ObjectID is the name/path of the object that triggered the event.
	ObjectID string

	// The generation number of the object that was overwritten by the object
	// that this notification pertains to. This attribute only appears in
	// OBJECT_FINALIZE events in the case of an overwrite.
	OverwroteGeneration string

	// MD5 is the MD5 hash of the object as a hex encoded string.
	MD5 string

	// TimeStamp is the time of the last update in Unix time (seconds since the epoch).
	TimeStamp int64
}

// NewStorageEvent is a convenience method to create a new StorageEvent. Currently all
// instances have storage.ObjectFinalizeEvent as GCSEventType. This indicates a new object
// being created.
func NewStorageEvent(bucketID, objectID string, lastUpdated int64, md5 string) *StorageEvent {
	return &StorageEvent{
		GCSEventType: storage.ObjectFinalizeEvent,
		BucketID:     bucketID,
		ObjectID:     objectID,
		TimeStamp:    lastUpdated,
		MD5:          md5,
	}
}

// MemEventBus implement the EventBus interface for an in-process event bus.
type MemEventBus struct {
	// Map of handlers keyed by channel. This is used to keep track of subscriptions.
	handlers map[string]channelHandler

	// concurrentPub is used to limit the number of go-routines that can concurrently
	// publish events. Since each Publish call can spin up multiple go-routines we avoid
	// creating too many. In most cases the maximum will never be reached.
	concurrentPub chan bool

	// Used to protect handlers.
	mutex sync.RWMutex

	// storageNotifications keep track of storage notifications. Mainly used for
	// testing with this implementation of EventBus.
	storageNotifications *NotificationsMap
}

// Internal type to keep track of the handlers for a single channel.
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
	// If this is a synthethic storage event then reframe it as an actual storage event.
	if channel == SYN_STORAGE_EVENT {
		evt := arg.(*StorageEvent)
		channelIDs := e.storageNotifications.Matches(evt.BucketID, evt.ObjectID)
		for _, channelID := range channelIDs {
			e.Publish(channelID, arg, true)
		}
		return
	}

	func() {
		e.mutex.RLock()
		defer e.mutex.RUnlock()
		if callbacks, ok := e.handlers[channel]; ok {
			for _, callback := range callbacks {
				e.concurrentPub <- true
				go func(callback CallbackFn) {
					defer func() { <-e.concurrentPub }()
					callback(arg)
				}(callback)
			}
		}
	}()
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
	notificationsID := GetNotificationID(bucketName, objectPrefix)
	return e.storageNotifications.Add(notificationsID, objectRegEx), nil
}

// PublishStorageEvent implements the EventBus interface.
func (e *MemEventBus) PublishStorageEvent(evtData *StorageEvent) {
	e.Publish(SYN_STORAGE_EVENT, evtData, true)
}

// NotificationsMap is a helper type that keep track of storage events.
// It is intended to be used by the MemEventBus and distEventBus (see gevent package)
// implementations of EventBus
//
// It assumes that storage events mainly consist of buckets and objects and
// related meta data.
//
// It uses the notion of a 'notification ID' which is a combination of a bucket and object prefix to
// keep track of server side storage events. Regular expressions are used to filter storage events
// on the client side.
// A channel ID (as defined by the EventBus interface) is a prefixed combination of the
// notification ID and a regular expression. For each notification ID (= a server side subscription
// to storage events) there can be an arbitrary number of regular expressions.
//
// NotificationsMap keeps track of the notification IDs and the associated regular expressions.
// It can then be used to match storage events against notification IDs (= subscriptions) and
// their regular expressions.
//
type NotificationsMap struct {
	// notifications maps notificationID -> map[string_repr_of_regexp]Regexp
	notifications map[string]map[string]*regexp.Regexp
	mutex         sync.Mutex
}

// GetNotificationID returns a string that is a combination of a bucket and an object prefix
// representing a server-side subscription to storage events.
func GetNotificationID(bucketName, objectPrefix string) string {
	return bucketName + "/" + strings.TrimLeft(objectPrefix, "/")
}

// NewNotifications creates a new instance of NotificationsMap
func NewNotificationsMap() *NotificationsMap {
	return &NotificationsMap{notifications: map[string]map[string]*regexp.Regexp{}}
}

// Add adds a notification to the map that consists of a notification id
// (created via GetNotificationID) and regular expression. The regex can be nil. If not nil,
// it will be used for client side filtering of object IDs that are delivered by events.
// It returns a channelID that should be used as the return value of the
// RegisterStorageEvents(...) method that called Add(...) in the first place.
func (n *NotificationsMap) Add(notifyID string, objectRegEx *regexp.Regexp) string {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	// If no regex was provided we add a single entry.
	regexStr := ""
	if objectRegEx != nil {
		regexStr = objectRegEx.String()
	}
	if _, ok := n.notifications[notifyID]; !ok {
		n.notifications[notifyID] = map[string]*regexp.Regexp{}
	}
	n.notifications[notifyID][regexStr] = objectRegEx
	return getChannelID(notifyID, objectRegEx)
}

// MatchesByID assumes that the given objectID matches the object prefix encoded in notification ID.
// This is usually the case when the objectID was delivered as a PubSub event together with the
// notificationID (see the gevent package as an example). It will then check whether the objectID
// matches the regular expressions associated with the notification id.
// It returns a list of channel IDs to which events should be sent.
func (n *NotificationsMap) MatchesByID(notificationID, objectID string) []string {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	// Find the notification ID. If it's not registered no channel IDs are returned.
	regexes, ok := n.notifications[notificationID]
	if !ok {
		return []string{}
	}
	return getChannelIDsFromRegexps(notificationID, objectID, regexes)
}

// Matches checks whether the given bucketID and objectID are in the recorded
// list of notifications and the regular expressions associated with them.
// It returns the channel IDs that match the found events.
func (n *NotificationsMap) Matches(bucketID, objectID string) []string {
	n.mutex.Lock()
	defer n.mutex.Unlock()

	// Iterate over all notifications and check if they match.
	ret := []string{}
	for notifyID, regexes := range n.notifications {
		notifyBucketID, objectPrefix := splitNotificationID(notifyID)
		if bucketID == notifyBucketID && strings.HasPrefix(objectID, objectPrefix) {
			ret = append(ret, getChannelIDsFromRegexps(notifyID, objectID, regexes)...)
		}
	}
	return ret
}

// getChannelID returns a unique channel id for the given pair of notificationID and
// regular expression. It will eventually be returned by an implementation of the
// RegisterStorageEvents method and can be used by the caller to subscribe to storage
// events connected to the channel.
func getChannelID(notificationID string, regEx *regexp.Regexp) string {
	regexStr := ""
	if regEx != nil {
		regexStr = regEx.String()
	}
	return storageChannelIDPrefix + notificationID + "/" + regexStr
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

// getChannelIDsFromRegexps check whether the given objectID matches the regular expressions
// and generates channel IDs. This assumes that the objectID has already been confirmed as
// matching the prefix encoded in the notificationID.
func getChannelIDsFromRegexps(notificationID, objectID string, regexes map[string]*regexp.Regexp) []string {
	// Check the objectID against the regular expressions.
	ret := make([]string, 0, len(regexes))
	for id, oneRegEx := range regexes {
		if id == "" || oneRegEx.MatchString(objectID) {
			ret = append(ret, getChannelID(notificationID, oneRegEx))
		}
	}
	return ret
}
