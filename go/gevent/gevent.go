package gevent

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sync"

	"cloud.google.com/go/pubsub"
	"cloud.google.com/go/storage"
	"github.com/davecgh/go-spew/spew"
	"google.golang.org/api/option"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// codecMap holds codecs for the different event channels. Values are added
// via the RegisterCodec function.
var codecMap = sync.Map{}

// RegisterCodec defines a codec for the given event channel.
func RegisterCodec(channel string, codec util.LRUCodec) {
	codecMap.Store(channel, codec)
}

// DistEventBus implements the eventbus.EventBus interface on top of Cloud PubSub.
type DistEventBus struct {
	localEventBus  eventbus.EventBus
	client         *pubsub.Client
	clientID       string
	projectID      string
	topicID        string
	topic          *pubsub.Topic
	sub            *pubsub.Subscription
	wrapperCodec   util.LRUCodec
	storageBuckets util.StringSet
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
		localEventBus: eventbus.New(),
		wrapperCodec:  util.JSONCodec(&channelWrapper{}),
		projectID:     projectID,
		topicID:       topicName,
	}

	// Create the client.
	var err error
	/// opts = append(opts, option.WithScopes(pubsub.ScopePubSub))
	ret.client, err = pubsub.NewClient(context.Background(), projectID)
	if err != nil {
		return nil, sklog.FmtErrorf("Error creating pubsub client: %s", err)
	}

	// Set up the pubsub client, topic and subscription.
	sklog.Infof("AAAA")
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

func (d *DistEventBus) RegisterStorageEvents(bucketName string, client *storage.Client) error {
	ctx := context.TODO()
	bucket := client.Bucket(bucketName)

	notifications, err := bucket.Notifications(ctx)
	if err != nil {
		return err
	}
	sklog.Infof("Retrieved: %d notifications", len(notifications))

	found := false
	for _, notify := range notifications {
		if notify.TopicID == d.topic.ID() {
			found = true
			break
		}
	}

	if !found {
		sklog.Infof("Adding notifications for %s %s %s", d.projectID, d.topic.ID(), bucketName)
		bucket := client.Bucket(bucketName)
		notifyID, err := bucket.AddNotification(ctx, &storage.Notification{
			TopicProjectID: d.projectID,
			TopicID:        d.topic.ID(),
			EventTypes:     []string{storage.ObjectFinalizeEvent},
			PayloadFormat:  storage.JSONPayload,
		})
		if err != nil {
			return sklog.FmtErrorf("Error registering event: %s", err)
		}
		sklog.Infof("notify: %s", spew.Sdump(notifyID))
	}

	d.storageBuckets[bucketName] = true
	return nil
}

func (d *DistEventBus) StorageEventType(bucketName string) string {
	return "--storage-event-" + bucketName
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
	wrapper, data, err := d.decodeMsg(msg)
	if err != nil {
		sklog.Errorf("Error decoding message: %s", err)
		return
	}
	// Publish the event locally if it hasn't been sent by this instance.
	if wrapper.Sender != d.clientID {
		d.localEventBus.Publish(wrapper.Channel, data, true)
	}
}

// decodeMsg unwraps the channelWrapper instance contained in the pubsub message
// and returns the deserialized payload as an instance of interface{}.
func (d *DistEventBus) decodeMsg(msg *pubsub.Message) (*channelWrapper, interface{}, error) {
	// If this
	wrapper, data, err := d.decodeStorageEvent(msg)
	if wrapper != nil || err != nil {
		return wrapper, data, err
	}

	// Unwrap the payload if this was wrapped in a channel wrapper.
	payload := msg.Data
	var codec util.LRUCodec = nil
	if d.wrapperCodec != nil {
		tempWrapper, err := d.wrapperCodec.Decode(payload)
		if err != nil {
			return nil, nil, fmt.Errorf("Error decoding message wrapper: %s", err)
		}
		wrapper = tempWrapper.(*channelWrapper)
		payload = wrapper.Data
		codecInst, ok := codecMap.Load(wrapper.Channel)
		if !ok {
			return nil, nil, fmt.Errorf("Unable to decode message for channel '%s'. No codec registered.", wrapper.Channel)
		}
		codec = codecInst.(util.LRUCodec)
	}

	// Deserialize the payload.
	data, err = codec.Decode(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to decode payload of pubsub event: %s", err)
	}
	return wrapper, data, nil
}

func (d *DistEventBus) decodeStorageEvent(msg *pubsub.Message) (*channelWrapper, interface{}, error) {
	// Test if this is a storage notification.
	if msg.Attributes["notificationConfig"] == "" {
		return nil, nil, nil
	}

	bucketName := msg.Attributes["bucketId"]
	if !d.storageBuckets[bucketName] {
		return nil, nil, sklog.FmtErrorf("Received event for unregistered storage bucket: %s", bucketName)
	}

	wrapper := &channelWrapper{Channel: d.StorageEventType(bucketName)}
	data := &storage.ObjectAttrs{}
	if err := json.Unmarshal([]byte(msg.Attributes[""]), &data); err != nil {
		return nil, nil, err
	}
	return wrapper, data, nil
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
