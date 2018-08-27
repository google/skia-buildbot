package gevent

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/davecgh/go-spew/spew"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	notificationIDAttr = "eventNotificationID"
	storageEventPrefix = "--storage-event-"
)

// codecMap holds codecs for the different event channels. Values are added
// via the RegisterCodec function.
var codecMap = sync.Map{}

// RegisterCodec defines a codec for the given event channel.
func RegisterCodec(channel string, codec util.LRUCodec) {
	codecMap.Store(channel, codec)
}

type StorageNotification struct {
	EventType string
	BucketID  string
	ObjectID  string
}

// DistEventBus implements the eventbus.EventBus interface on top of Cloud PubSub.
type DistEventBus struct {
	localEventBus        eventbus.EventBus
	client               *pubsub.Client
	clientID             string
	projectID            string
	topicID              string
	topic                *pubsub.Topic
	sub                  *pubsub.Subscription
	wrapperCodec         util.LRUCodec
	storageNotifications map[string]map[string]*regexp.Regexp
	storageNotifyMutex   sync.Mutex
}

// channelWrapper wraps each message to do channel multiplexing on top of a
// Cloud PubSub topic.
type channelWrapper struct {
	Sender  string `json:"sender"`    // id of the sending node.
	Channel string `json:"eventType"` // event channel of this message.
	Data    []byte `json:"data"`      // payload encoded with the user supplied codec.
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
	ret := &DistEventBus{
		localEventBus:        eventbus.New(),
		wrapperCodec:         util.JSONCodec(&channelWrapper{}),
		storageNotifications: map[string]map[string]*regexp.Regexp{},
		projectID:            projectID,
		topicID:              topicName,
	}

	// Create the client.
	var err error
	/// opts = append(opts, option.WithScopes(pubsub.ScopePubSub))
	ret.client, err = pubsub.NewClient(context.Background(), projectID)
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
func (d *DistEventBus) Publish(channel string, arg interface{}, globally bool) {
	if globally {
		// publish to pubsub in the background.
		go func() {
			codecInstance, ok := codecMap.Load(channel)
			if !ok {
				sklog.Errorf("Unable to publish on channel '%s'. No codec defined.", channel)
				return
			}

			msg, err := d.encodeMsg(channel, arg, codecInstance.(util.LRUCodec))
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
	d.localEventBus.Publish(channel, arg, false)
}

// SubscribeAsync implements the eventbus.EventBus interface.
func (d *DistEventBus) SubscribeAsync(eventType string, callback eventbus.CallbackFn) {
	d.localEventBus.SubscribeAsync(eventType, callback)
}

func (d *DistEventBus) RegisterStorageEvents(bucketName string, objectPrefix string, objectRegEx *regexp.Regexp, client *storage.Client) (string, error) {
	d.storageNotifyMutex.Lock()
	defer d.storageNotifyMutex.Unlock()

	ctx := context.TODO()
	bucket := client.Bucket(bucketName)

	notifications, err := bucket.Notifications(ctx)
	if err != nil {
		return "", err
	}
	sklog.Infof("Retrieved: %d notifications", len(notifications))

	found := false
	notifyID := bucketName + "/" + strings.TrimLeft(objectPrefix, "/")
	for _, notify := range notifications {
		sklog.Infof("Got notification: \n%s\n", spew.Sdump(notify))
		if notify.TopicID == d.topic.ID() && notify.ObjectNamePrefix == objectPrefix {
			// If we don't have the custom notification attribute we want to create new
			// subscription since this might be from a different process.
			if notify.CustomAttributes[notificationIDAttr] != notifyID {
				continue
			}
			found = true
			break
		}
	}

	if !found {
		sklog.Infof("Adding notifications for %s %s %s", d.projectID, d.topic.ID(), bucketName)
		bucket := client.Bucket(bucketName)
		newNotification, err := bucket.AddNotification(ctx, &storage.Notification{
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
		sklog.Infof("notify: %s", spew.Sdump(newNotification))
	}

	// If no regex was provided we add a single entry.
	regexStr := ""
	if objectRegEx != nil {
		regexStr = objectRegEx.String()
	}
	if _, ok := d.storageNotifications[notifyID]; !ok {
		d.storageNotifications[notifyID] = map[string]*regexp.Regexp{}
	}
	d.storageNotifications[notifyID][regexStr] = objectRegEx
	return getEventType(notifyID, objectRegEx), nil
}

func getEventType(notificationID string, regEx *regexp.Regexp) string {
	regexStr := ""
	if regEx != nil {
		regexStr = regEx.String()
	}
	return storageEventPrefix + notificationID + "/" + regexStr
}

// setupTopicSub sets up the topic and subscription.
func (d *DistEventBus) setupTopicSub(topicName, subscriberName string) error {
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
	// Make the subscription also the id of this client.
	d.clientID = subName
	return nil
}

// startReceiver start a goroutine that processes incoming pubsub messages
// and fires events on this node.
func (d *DistEventBus) startReceiver() {
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
func (d *DistEventBus) processReceivedMsg(ctx context.Context, msg *pubsub.Message) {
	defer msg.Ack()
	wrappers, data, ignore, err := d.decodeMsg(msg)
	if err != nil {
		sklog.Errorf("Error decoding message: %s", err)
		return
	}

	if ignore {
		return
	}

	// Publish the events locally if it hasn't been sent by this instance.
	for _, wrapper := range wrappers {
		if wrapper.Sender != d.clientID {
			d.localEventBus.Publish(wrapper.Channel, data, true)
		}
	}
}

// decodeMsg unwraps the channelWrapper instance contained in the pubsub message
// and returns the deserialized payload as an instance of interface{}.
func (d *DistEventBus) decodeMsg(msg *pubsub.Message) ([]*channelWrapper, interface{}, bool, error) {
	// sklog.Infof("XXX: %s", spew.Sdump(msg))

	// Check if this is a storage event.
	wrappers, data, ignore, err := d.decodeStorageEvent(msg)
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
		codecInst, ok := codecMap.Load(wrapper.Channel)
		if !ok {
			return nil, nil, false, fmt.Errorf("Unable to decode message for channel '%s'. No codec registered.", wrapper.Channel)
		}
		codec = codecInst.(util.LRUCodec)
	}

	// Deserialize the payload.
	data, err = codec.Decode(payload)
	if err != nil {
		return nil, nil, false, fmt.Errorf("Unable to decode payload of pubsub event: %s", err)
	}
	return []*channelWrapper{wrapper}, data, false, nil
}

func (d *DistEventBus) decodeStorageEvent(msg *pubsub.Message) ([]*channelWrapper, interface{}, bool, error) {
	// Test if this is a storage notification. If no then we are done.
	if msg.Attributes["notificationConfig"] == "" {
		return nil, nil, false, nil
	}

	d.storageNotifyMutex.Lock()
	defer d.storageNotifyMutex.Unlock()

	bucketID := msg.Attributes["bucketId"]
	objectID := msg.Attributes["objectId"]
	notificationID := msg.Attributes[notificationIDAttr]
	sklog.Infof("ID : %s    %s      %s", notificationID, bucketID, objectID)

	regexes, ok := d.storageNotifications[notificationID]
	if !ok {
		return nil, nil, false, sklog.FmtErrorf("Received event for unregistered storage bucket (%s) and object (%s) with id: '%s'", bucketID, objectID, notificationID)
	}

	data := &StorageNotification{
		EventType: msg.Attributes["eventType"],
		BucketID:  bucketID,
		ObjectID:  objectID,
	}

	wrappers := make([]*channelWrapper, 0, len(regexes))
	for id, oneRegEx := range regexes {
		if id == "" || oneRegEx.Match([]byte(objectID)) {
			wrappers = append(wrappers, &channelWrapper{
				Channel: getEventType(notificationID, oneRegEx),
			})
			sklog.Infof("WRAPPER: %s", getEventType(notificationID, oneRegEx))
		}
	}
	sklog.Infof("wrappers: %d", len(wrappers))

	return wrappers, data, false, nil
}

// encodeMsg wraps the given payload into an instance of channelWrapper and
// creates the necessary pubsub message to send it to the cloud.
func (d *DistEventBus) encodeMsg(channel string, data interface{}, codec util.LRUCodec) (*pubsub.Message, error) {
	payload, err := codec.Encode(data)
	if err != nil {
		return nil, err
	}

	if d.wrapperCodec != nil {
		wrapper := &channelWrapper{
			Sender:  d.sub.ID(),
			Channel: channel,
			Data:    payload,
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
