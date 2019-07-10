package gevent

import (
	"context"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sync"
	"time"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/davecgh/go-spew/spew"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/option"
)

const (
	// notificationIDAttr is the name of the custom attribute in storage events
	// is injected to connect it with registrations issued from distEventBus.
	notificationIDAttr = "eventNotificationID"

	// When subscribed to a pubsub topic, this is set as the number of MaxOutstandingMessages.
	// This effectively limits how many Pubsub notifications this instance can handle at
	// once. If this instance is working on this number or more, the pubsub client won't
	// assign it any more. This number is kept small to reduce RAM and goroutine usage.
	// Furthermore, it reduces the number of "lost" Pubsub notifications that will
	// have to be re-tried elsewhere if this instance dies.
	MaximumConcurrentPublishesPerTopic = 100
)

func init() {
	// Register a codec for synthetic storage events.
	RegisterCodec(eventbus.SYN_STORAGE_EVENT, util.JSONCodec(&eventbus.StorageEvent{}))
}

// codecMap holds codecs for the different event channels. Values are added
// via the RegisterCodec function.
var codecMap = sync.Map{}

// RegisterCodec defines a codec for the given event channel.
func RegisterCodec(channelID string, codec util.LRUCodec) {
	codecMap.Store(channelID, codec)
}

// distEventBus implements the eventbus.EventBus interface on top of Cloud PubSub.
type distEventBus struct {
	localEventBus eventbus.EventBus
	client        *pubsub.Client
	clientID      string
	projectID     string
	topicID       string
	topic         *pubsub.Topic
	sub           *pubsub.Subscription
	wrapperCodec  util.LRUCodec

	// storageNotifications keep track of storage events we have subscribed to.
	// See eventbus.NotificationsMap for details.
	storageNotifications *eventbus.NotificationsMap

	// disableGCSSubscriptions disables registrations of storage events for testing.
	disableGCSSubscriptions bool
}

// channelWrapper wraps each message to do channel multiplexing on top of a
// Cloud PubSub topic.
type channelWrapper struct {
	Sender    string `json:"sender"`    // id of the sending node.
	ChannelID string `json:"channelID"` // event channel of this message.
	Data      []byte `json:"data"`      // payload encoded with the user supplied codec.
}

// New returns an instance of eventbus.EventBus that is a node in a distributed
// eventbus.
// Each instance is a node in a distributed event bus that allows to send events
// on an arbitrary number of channels.
// - projectID is the id of the GCP project where the PubSub topic should live.
// - topicName is the topic to use. It is assume that all message on this topic
//   are messages of the
//   event bus.
// - subscriberName is an id that uniquely identifies this node within the
//   event bus network.
// - opts are the options used to create an authenticated PubSub client.
func New(projectID, topicName, subscriberName string, opts ...option.ClientOption) (eventbus.EventBus, error) {
	ret := &distEventBus{
		localEventBus:        eventbus.New(),
		wrapperCodec:         util.JSONCodec(&channelWrapper{}),
		storageNotifications: eventbus.NewNotificationsMap(),
		projectID:            projectID,
		topicID:              topicName,
	}

	// Create the client.
	var err error
	ret.client, err = pubsub.NewClient(context.Background(), projectID, opts...)
	if err != nil {
		return nil, sklog.FmtErrorf("Error creating pubsub client: %s", err)
	}

	// Set up the pubsub client, topic and subscription.
	if err := ret.setupTopicSub(topicName, subscriberName); err != nil {
		return nil, err
	}

	// Start the receiver.
	ret.startReceiver()
	return ret, nil
}

// Publish implements the eventbus.EventBus interface.
func (d *distEventBus) Publish(channelID string, arg interface{}, globally bool) {
	if globally {
		// publish to pubsub in the background.
		go func() {
			codecInstance, ok := codecMap.Load(channelID)
			if !ok {
				sklog.Errorf("Unable to publish on channel '%s'. No codec defined.", channelID)
				return
			}

			msg, err := d.encodeMsg(channelID, arg, codecInstance.(util.LRUCodec))
			if err != nil {
				sklog.Errorf("Error encoding outgoing message: %s", err)
				return
			}
			ctx := context.Background()
			pubResult := d.topic.Publish(ctx, msg)
			if _, err = pubResult.Get(ctx); err != nil {
				sklog.Errorf("Error publishing message: %s", err)
				return
			}
		}()
	}
	// Publish the event locally.
	d.localEventBus.Publish(channelID, arg, false)
}

// SubscribeAsync implements the eventbus.EventBus interface.
func (d *distEventBus) SubscribeAsync(channelID string, callback eventbus.CallbackFn) {
	d.localEventBus.SubscribeAsync(channelID, callback)
}

// RegisterStorageEvents implements the eventbus.EventBus interface.
func (d *distEventBus) RegisterStorageEvents(bucketName string, objectPrefix string, objectRegEx *regexp.Regexp, client *storage.Client) (string, error) {
	ctx := context.TODO()
	bucket := client.Bucket(bucketName)

	notifyID := eventbus.GetNotificationID(bucketName, objectPrefix)
	if !d.disableGCSSubscriptions {
		notifications, err := bucket.Notifications(ctx)
		if err != nil {
			return "", err
		}
		sklog.Infof("Retrieved: %d notifications", len(notifications))

		var notificationInfo *storage.Notification
		found := false
		for _, notify := range notifications {
			if notify.TopicID == d.topic.ID() && notify.ObjectNamePrefix == objectPrefix {
				// If we don't have the custom notification attribute we want to create new
				// subscription since this might be from a different process.
				if notify.CustomAttributes[notificationIDAttr] != notifyID {
					continue
				}
				notificationInfo = notify
				found = true
				break
			}
		}

		if !found {
			bucket := client.Bucket(bucketName)
			notificationInfo, err = bucket.AddNotification(ctx, &storage.Notification{
				TopicProjectID:   d.projectID,
				TopicID:          d.topic.ID(),
				EventTypes:       []string{storage.ObjectFinalizeEvent},
				PayloadFormat:    storage.JSONPayload,
				ObjectNamePrefix: objectPrefix,
				CustomAttributes: map[string]string{
					notificationIDAttr: notifyID,
				},
			})
			if err != nil {
				return "", sklog.FmtErrorf("Error registering event: %s", err)
			}
			sklog.Infof("Created storage notification: %s", spew.Sdump(notificationInfo))
		} else {
			sklog.Infof("Re-using storage notification: %s", spew.Sdump(notificationInfo))
		}
	}

	// Register the same event in the local event bus because local events are directly published there.
	if _, err := d.localEventBus.RegisterStorageEvents(bucketName, objectPrefix, objectRegEx, nil); err != nil {
		return "", err
	}
	return d.storageNotifications.Add(notifyID, objectRegEx), nil
}

// PublishStorageEvent implements the EventBus interface.
func (d *distEventBus) PublishStorageEvent(evtData *eventbus.StorageEvent) {
	d.Publish(eventbus.SYN_STORAGE_EVENT, evtData, true)
}

// setupTopicSub sets up the topic and subscription.
func (d *distEventBus) setupTopicSub(topicName, subscriberName string) error {
	ctx := context.Background()

	// Create the topic if it doesn't exist yet.
	d.topic = d.client.Topic(topicName)
	if exists, err := d.topic.Exists(ctx); err != nil {
		return sklog.FmtErrorf("Error checking whether topic exits: %s", err)
	} else if !exists {
		if d.topic, err = d.client.CreateTopic(ctx, topicName); err != nil {
			return sklog.FmtErrorf("Error creating pubsub topic '%s': %s", topicName, err)
		}
	}

	// Create the subscription if it doesn't exist.
	subName := fmt.Sprintf("%s+%s", subscriberName, topicName)
	d.sub = d.client.Subscription(subName)
	if exists, err := d.sub.Exists(ctx); err != nil {
		return sklog.FmtErrorf("Error checking existence of pubsub subscription '%s': %s", subName, err)
	} else if !exists {
		d.sub, err = d.client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic: d.topic,
		})
		if err != nil {
			return sklog.FmtErrorf("Error creating pubsub subscription '%s': %s", subName, err)
		}
	}
	d.sub.ReceiveSettings.MaxOutstandingMessages = MaximumConcurrentPublishesPerTopic
	// Make the subscription also the id of this client.
	d.clientID = subName
	return nil
}

// startReceiver start a goroutine that processes incoming pubsub messages
// and fires events on this node.
func (d *distEventBus) startReceiver() {
	go func() {
		ctx := context.Background()
		for {
			err := d.sub.Receive(ctx, d.processReceivedMsg)
			if err != nil {
				sklog.Errorf("Error receiving message: %s", err)
				continue
			}
		}
	}()
}

// processReceivedMsg handles each pubsub message that arrives. It unwraps the
// enclosed channelWrapper and dispatches the event in this process unless the
// received message was sent by this node.
func (d *distEventBus) processReceivedMsg(ctx context.Context, msg *pubsub.Message) {
	defer msg.Ack()
	wrappers, data, ignore, err := d.decodeMsg(msg)
	if err != nil {
		sklog.Errorf("Error decoding message: %s", err)
		return
	}

	// If this was flagged to ignore then we are done.
	if ignore {
		return
	}

	// Publish the events locally if it hasn't been sent by this instance.
	for _, wrapper := range wrappers {
		if wrapper.Sender != d.clientID {
			d.localEventBus.Publish(wrapper.ChannelID, data, true)
		}
	}
}

// decodeMsg unwraps the pubsub message and returns meta information
// (as channelWrapper messages) about the channels where the data should be sent.
// The deserialized payload is returned as an instance of interface{}.
// The third return value indicates wether to ignore the message. It it
// returns 'true' no event should be dispatched.
func (d *distEventBus) decodeMsg(msg *pubsub.Message) ([]*channelWrapper, interface{}, bool, error) {
	// Check if this is a storage event.
	wrappers, data, ignore, err := d.decodeStorageMsg(msg)
	if ignore || wrappers != nil || err != nil {
		return wrappers, data, ignore, err
	}

	// Unwrap the payload if this was wrapped in a channel wrapper.
	payload := msg.Data
	var codec util.LRUCodec = nil
	var wrapper *channelWrapper
	if d.wrapperCodec != nil {
		tempWrapper, err := d.wrapperCodec.Decode(payload)
		if err != nil {
			return nil, nil, false, fmt.Errorf("Error decoding message wrapper: %s", err)
		}
		wrapper = tempWrapper.(*channelWrapper)
		payload = wrapper.Data
		codecInst, ok := codecMap.Load(wrapper.ChannelID)
		if !ok {
			return nil, nil, false, fmt.Errorf("Unable to decode message for channel '%s'. No codec registered.", wrapper.ChannelID)
		}
		codec = codecInst.(util.LRUCodec)
	}

	// Deserialize the payload.
	data, err = codec.Decode(payload)
	if err != nil {
		return nil, nil, false, fmt.Errorf("Unable to decode payload of pubsub event: %s", err)
	}

	// Check if this is a synthetic storage event in which case we need to notify the right subscribers.
	if wrapper.ChannelID == eventbus.SYN_STORAGE_EVENT {
		return d.wrapSyntheticStorageEvent(wrapper, data)
	}

	return []*channelWrapper{wrapper}, data, false, nil
}

// wrapSyntheticStorageEvent takes a synthetic storage event and translates it into
// valid storage events based on subscriptions.
func (d *distEventBus) wrapSyntheticStorageEvent(wrapper *channelWrapper, evtData interface{}) ([]*channelWrapper, interface{}, bool, error) {
	evt := evtData.(*eventbus.StorageEvent)
	wrappers := []*channelWrapper{}
	channelIDs := d.storageNotifications.Matches(evt.BucketID, evt.ObjectID)
	for _, channelID := range channelIDs {
		newWrapper := *wrapper
		newWrapper.ChannelID = channelID
		wrappers = append(wrappers, &newWrapper)
	}

	return wrappers, evtData, false, nil
}

// objectAttrs is a helper struct to parse the object attributes that are
// delivered with storage events.
// Note: Using the GCS package (storage.ObjectAttrs) is not an option because
// it does not implement parsing JSON. So we only parse the attributes we are
// interested in.
type objectAttrs struct {
	Bucket               string    `json:"bucket"`
	Name                 string    `json:"name"`
	Updated              time.Time `json:"updated"`
	Base64EncodedMD5Hash string    `json:"md5Hash"` // base64 encoded MD5 hash
}

// decodeStorageMsg checks wether the given pubsub message is a notification
// from a storage event. If not all return values will be nil value.
// Otherwise the return values match the return values of decodeMsg.
func (d *distEventBus) decodeStorageMsg(msg *pubsub.Message) ([]*channelWrapper, interface{}, bool, error) {
	// Test if this is a storage notification. If no then we are done.
	if msg.Attributes["notificationConfig"] == "" {
		return nil, nil, false, nil
	}

	bucketID := msg.Attributes["bucketId"]
	objectID := msg.Attributes["objectId"]
	notificationID := msg.Attributes[notificationIDAttr]

	channelIDs := d.storageNotifications.MatchesByID(notificationID, objectID)
	if len(channelIDs) == 0 {
		// Ignore events that have not been registered. Not all clients register for
		// all events.
		return nil, nil, true, nil
	}

	// Extract the object attributes.
	attrs := &objectAttrs{}
	if err := json.Unmarshal(msg.Data, attrs); err != nil {
		return nil, nil, false, sklog.FmtErrorf("Unable to decode object attributes: %s", err)
	}

	// decode the MD5 hash: base64 -> bytes -> hex-encoded-string
	md5Bytes, err := base64.StdEncoding.DecodeString(attrs.Base64EncodedMD5Hash)
	if err != nil {
		return nil, nil, false, sklog.FmtErrorf("Unable to decode base64 encoded MD5 hash (%s): %s", attrs.Base64EncodedMD5Hash, err)
	}
	md5HashStr := hex.EncodeToString(md5Bytes)

	data := &eventbus.StorageEvent{
		GCSEventType: msg.Attributes["eventType"],
		BucketID:     bucketID,
		ObjectID:     objectID,
		TimeStamp:    attrs.Updated.Unix(),
		MD5:          md5HashStr,

		// Only appears in OBJECT_FINALIZE events in the case of an overwrite.
		OverwroteGeneration: msg.Attributes["overwroteGeneration"],
	}

	wrappers := make([]*channelWrapper, 0, len(channelIDs))
	for _, channelID := range channelIDs {
		wrappers = append(wrappers, &channelWrapper{ChannelID: channelID})
	}
	return wrappers, data, false, nil
}

// encodeMsg wraps the given payload into an instance of channelWrapper and
// creates the necessary pubsub message to send it to the cloud.
func (d *distEventBus) encodeMsg(channelID string, data interface{}, codec util.LRUCodec) (*pubsub.Message, error) {
	payload, err := codec.Encode(data)
	if err != nil {
		return nil, err
	}

	if d.wrapperCodec != nil {
		wrapper := &channelWrapper{
			Sender:    d.sub.ID(),
			ChannelID: channelID,
			Data:      payload,
		}
		var err error
		if payload, err = d.wrapperCodec.Encode(wrapper); err != nil {
			return nil, err
		}
	}
	return &pubsub.Message{
		Data: payload,
	}, nil
}

// GetNodeName generates a service name for this host based on the hostname and
// whether we are running locally or in the cloud. This is enough to distinguish
// between hosts and can be used across services, e.g. pubsub subscription or
// logging and tracing information. appName is usually the name of the executable
// calling the function.
func GetNodeName(appName string, local bool) (string, error) {
	hostName, err := os.Hostname()
	if err != nil {
		return "", err
	}

	retHostName := hostName
	if local {
		retHostName = "local-" + hostName
	}

	return appName + "-" + retHostName, nil
}
