package gevent

import (
	"context"
	"fmt"
	"sync"

	"cloud.google.com/go/pubsub"

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

// DistEventBus implements the eventbus.EventBus interfase on top of Cloud PubSub.
type distEventBus struct {
	localEventBus eventbus.EventBus
	client        *pubsub.Client
	clientID      string
	topic         *pubsub.Topic
	sub           *pubsub.Subscription
	wrapperCodec  util.LRUCodec
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
func New(projectID, topicName, subscriberName string) (eventbus.EventBus, error) {
	ret := &distEventBus{
		localEventBus: eventbus.New(),
		wrapperCodec:  util.JSONCodec(&channelWrapper{}),
	}

	// Set up the pubsub client, topic and subscription.
	if err := ret.setupClientTopicSub(projectID, topicName, subscriberName); err != nil {
		return nil, err
	}

	// Start the receiver.
	ret.startReceiver()
	return ret, nil
}

// Publish implements the eventbus.EventBus interface.
func (d *distEventBus) Publish(channel string, arg interface{}, globally bool) {
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
func (d *distEventBus) SubscribeAsync(eventType string, callback eventbus.CallbackFn) {
	d.localEventBus.SubscribeAsync(eventType, callback)
}

// setupclientTopicSub sets up the pubsub client, topic and subscription.
func (d *distEventBus) setupClientTopicSub(projectID, topicName, subscriberName string) error {
	ctx := context.Background()

	// Create a client.
	var err error
	d.client, err = pubsub.NewClient(ctx, projectID)
	if err != nil {
		return fmt.Errorf("Error creating pubsub client: %s", err)
	}

	// Create the topic if it doesn't exist yet.
	d.topic = d.client.Topic(topicName)
	if exists, err := d.topic.Exists(ctx); err != nil {
		return err
	} else if !exists {
		if d.topic, err = d.client.CreateTopic(ctx, topicName); err != nil {
			return fmt.Errorf("Error creating pubsub topic '%s': %s", topicName, err)
		}
	}

	// Create the subscription if it doesn't exist.
	subName := fmt.Sprintf("%s+%s", subscriberName, topicName)
	d.sub = d.client.Subscription(subName)
	if exists, err := d.sub.Exists(ctx); err != nil {
		return fmt.Errorf("Error checking existence of pubsub subscription '%s': %s", subName, err)
	} else if !exists {
		d.sub, err = d.client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic: d.topic,
		})
		if err != nil {
			return fmt.Errorf("Error creating pubsub subscription '%s': %s", subName, err)
		}
	}
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

// processReceivedMsg handles each pubsub message that arrives. It unwrapps the
// enclosed channelWrapper and dispatches the event in this process unless the
// received message was sent by this node.
func (d *distEventBus) processReceivedMsg(ctx context.Context, msg *pubsub.Message) {
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
func (d *distEventBus) decodeMsg(msg *pubsub.Message) (*channelWrapper, interface{}, error) {
	// Unwrap the payload if this was wrapped in a channel wrapper.
	var wrapper *channelWrapper = nil
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
	data, err := codec.Decode(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to decode payload of pubsub event: %s", err)
	}
	return wrapper, data, nil
}

// encodeMsg wraps the given payload into an instance of channelWrapper and
// creates the necessary pubsub message to send it to the cloud.
func (d *distEventBus) encodeMsg(channel string, data interface{}, codec util.LRUCodec) (*pubsub.Message, error) {
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
