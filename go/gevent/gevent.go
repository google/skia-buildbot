package gevent

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"

	"github.com/gorilla/mux"

	"golang.org/x/net/context"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
)

const (
	PUBSUB_FULLY_QUALIFIED_TOPIC_TMPL    = "projects/%s/topics/%s"
	PUBSUB_TOPIC_SWARMING_TASKS          = "swarming-tasks"
	PUBSUB_TOPIC_SWARMING_TASKS_INTERNAL = "swarming-tasks-internal"
	PUSH_URL_SWARMING_TASKS              = "pubsub/swarming-tasks"
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

func New(projectID, topicName, subscriberName string, codec util.LRUCodec) (eventbus.EventBus, error) {
	ctx := context.Background()

	// Create a client.
	client, err := pubsub.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Create topic and subscription if necessary.

	// Create the topic if it doesn't exist yet.
	topic := client.Topic(topicName)
	if exists, err := topic.Exists(ctx); err != nil {
		return nil, err
	} else if !exists {
		if topic, err = client.CreateTopic(ctx, topicName); err != nil {
			return nil, err
		}
	}

	// Create the subscription if it doesn't exist.
	subName := fmt.Sprintf("%s+%s", subscriberName, topicName)
	sub := client.Subscription(subName)
	if exists, err := sub.Exists(ctx); err != nil {
		return nil, err
	} else if !exists {
		sub, err = client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic:       topic,
			AckDeadline: 5 * time.Second,
		})
		if err != nil {
			return nil, err
		}
	}

	ret := &DistEventBus{
		memEventBus:  eventbus.New().(*eventbus.MemEventBus),
		client:       client,
		clientID:     subName,
		topic:        topic,
		sub:          sub,
		codec:        codec,
		wrapperCodec: util.JSONCodec(&channelWrapper{}),
	}

	// Start a goroutine to handle incoming messages.
	go func() {
		ctx := context.Background()
		for {
			err := sub.Receive(ctx, ret.processReceivedMsg)
			if err != nil {
				sklog.Errorf("Error receiving message: %s", err)
				continue
			}
		}
	}()

	return ret, nil
}

type channelWrapper struct {
	Sender  string `json:"sender"`
	Channel string `json:"eventType"`
	Data    []byte `json:"data"`
}

func (d *DistEventBus) processReceivedMsg(ctx context.Context, msg *pubsub.Message) {
	wrapper, data, err := d.decodeMsg(msg)
	if err != nil {
		sklog.Errorf("Error decoding message: %s", err)
	}
	// Publish the event locally if it hasn't been sent by this instance.
	if wrapper.Sender != d.clientID {
		d.memEventBus.Publish(wrapper.Channel, data)
	}
	msg.Ack()
}

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
	d.memEventBus.Publish(channel, arg)
}

func (d *DistEventBus) SubscribeAsync(eventType string, callback eventbus.CallbackFn) {
	d.memEventBus.SubscribeAsync(eventType, callback)
}

// InitPubSub ensures that the pub/sub topics and subscriptions needed by the
// TaskScheduler exist.
func InitPubSub(serverUrl, topicName, subscriberName string) error {
	ctx := context.Background()

	// Create a client.
	client, err := pubsub.NewClient(ctx, common.PROJECT_ID)
	if err != nil {
		return err
	}

	// Create topic and subscription if necessary.

	// Topic.
	topic := client.Topic(topicName)
	exists, err := topic.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		if _, err := client.CreateTopic(ctx, topicName); err != nil {
			return err
		}
	}

	// Subscription.
	subName := fmt.Sprintf("%s+%s", subscriberName, topicName)
	sub := client.Subscription(subName)
	exists, err = sub.Exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		endpoint := serverUrl
		if !strings.HasSuffix(endpoint, "/") {
			endpoint += "/"
		}
		endpoint += PUSH_URL_SWARMING_TASKS
		if _, err := client.CreateSubscription(ctx, subName, pubsub.SubscriptionConfig{
			Topic:       topic,
			AckDeadline: 3 * time.Minute,
			PushConfig: pubsub.PushConfig{
				Endpoint: endpoint,
			},
		}); err != nil {
			return err
		}
	}
	return nil
}

// PubSubRequest is the format of pub/sub HTTP request body.
type PubSubRequest struct {
	Message      pubsub.Message `json:"message"`
	Subscription string         `json:"subscription"`
}

// PubSubTaskMessage is a message received from Swarming via pub/sub about a
// Task.
type PubSubTaskMessage struct {
	SwarmingTaskId string `json:"task_id"`
}

// PubSubHandler is an interface used for handling pub/sub messages.
type PubSubHandler interface {
	HandleSwarmingPubSub(string) bool
}

// RegisterPubSubServer adds handler to r that handle pub/sub push
// notifications.
func RegisterPubSubServer(h PubSubHandler, r *mux.Router) {
	r.HandleFunc("/"+PUSH_URL_SWARMING_TASKS, func(w http.ResponseWriter, r *http.Request) {
		var req PubSubRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputils.ReportError(w, r, err, "Failed to decode request body.")
			return
		}

		var t PubSubTaskMessage
		if err := json.Unmarshal(req.Message.Data, &t); err != nil {
			httputils.ReportError(w, r, err, "Failed to decode PubSubTaskMessage.")
			return
		}

		ack := h.HandleSwarmingPubSub(t.SwarmingTaskId)
		if ack {
			w.WriteHeader(http.StatusOK)
		} else {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	}).Methods(http.MethodPost)
}
