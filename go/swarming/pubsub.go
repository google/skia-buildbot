package swarming

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

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
		c := &pubsub.PushConfig{
			Endpoint: endpoint,
		}
		if _, err := client.CreateSubscription(ctx, subName, topic, 3*time.Minute, c); err != nil {
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
