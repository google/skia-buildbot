package gevent

import (
	"fmt"
	"time"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"

	"golang.org/x/net/context"

	"cloud.google.com/go/pubsub"
)

type DistEventBus struct {
	memEventBus  *eventbus.MemEventBus
	client       *pubsub.Client
	clientID     string
	topic        *pubsub.Topic
	sub          *pubsub.Subscription
	codec        util.LRUCodec
	wrapperCodec util.LRUCodec
}

type channelWrapper struct {
	Sender  string `json:"sender"`
	Channel string `json:"eventType"`
	Data    []byte `json:"data"`
}

func New(projectID, topicName, subscriberName string, codec util.LRUCodec) (eventbus.EventBus, error) {
	ret := &DistEventBus{
		memEventBus:  eventbus.New().(*eventbus.MemEventBus),
		codec:        codec,
		wrapperCodec: util.JSONCodec(&channelWrapper{}),
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
func (d *DistEventBus) Publish(channel string, arg interface{}) {
	// publish to pubsub in the background.
	go func() {
		msg, err := d.encodeMsg(channel, arg)
		if err != nil {
			sklog.Errorf("Error encoding outgoing message: %s", err)
			return
		}
		ctx := context.Background()
		pubResult := d.topic.Publish(ctx, msg)
		_, err = pubResult.Get(ctx)
		if err != nil {
			sklog.Errorf("Error publishing message: %s", err)
			return
		}
	}()
	// Publish the event locally.
	d.memEventBus.Publish(channel, arg)
}

// SubscribeAsync implements the eventbus.EventBus interface.
func (d *DistEventBus) SubscribeAsync(eventType string, callback eventbus.CallbackFn) {
	d.memEventBus.SubscribeAsync(eventType, callback)
}

// setupclientTopicSub sets up the pubsub client, topic and subscription.
func (d *DistEventBus) setupClientTopicSub(projectID, topicName, subscriberName string) error {
	ctx := context.Background()

	// Create a client.
	var err error
	d.client, err = pubsub.NewClient(ctx, projectID)
	if err != nil {
		return err
	}

	// Create the topic if it doesn't exist yet.
	d.topic = d.client.Topic(topicName)
	if exists, err := d.topic.Exists(ctx); err != nil {
		return err
	} else if !exists {
		if d.topic, err = d.client.CreateTopic(ctx, topicName); err != nil {
			return err
		}
	}

	// Create the subscription if it doesn't exist.
	subName := fmt.Sprintf("%s+%s", subscriberName, topicName)
	d.sub = d.client.Subscription(subName)
	if exists, err := d.sub.Exists(ctx); err != nil {
		return err
	} else if !exists {
		d.sub, err = d.client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic:       d.topic,
			AckDeadline: 10 * time.Second,
		})
		if err != nil {
			return err
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

func (d *DistEventBus) processReceivedMsg(ctx context.Context, msg *pubsub.Message) {
	// sklog.Infof("Processing received message: %s", string(msg.Data))
	defer msg.Ack()
	wrapper, data, err := d.decodeMsg(msg)
	if err != nil {
		sklog.Errorf("Error decoding message: %s", err)
		return
	}
	// Publish the event locally if it hasn't been sent by this instance.
	if wrapper.Sender != d.clientID {
		d.memEventBus.Publish(wrapper.Channel, data)
	}
}

// decodeMsg unwraps the channelWrapper instance contained in the pubsub message
// and returns the deserialized payload as an instance of interface{}.
func (d *DistEventBus) decodeMsg(msg *pubsub.Message) (*channelWrapper, interface{}, error) {
	// Unwrap the payload if this was wrapped in a channel wrapper.
	var wrapper *channelWrapper = nil
	payload := msg.Data
	if d.wrapperCodec != nil {
		tempWrapper, err := d.wrapperCodec.Decode(payload)
		if err != nil {
			return nil, nil, fmt.Errorf("Error decoding message wrapper: %s", err)
		}
		wrapper = tempWrapper.(*channelWrapper)
		payload = wrapper.Data
	}

	// Deserialize the payload.
	data, err := d.codec.Decode(payload)
	if err != nil {
		return nil, nil, fmt.Errorf("Unable to decode payload of pubsub event: %s", err)
	}
	return wrapper, data, nil
}

// encodeMsg wraps the given payload into an instance of channelWrapper and
// creates the necessary pubsub message to send it to the cloud.
func (d *DistEventBus) encodeMsg(channel string, data interface{}) (*pubsub.Message, error) {
	payload, err := d.codec.Encode(data)
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
