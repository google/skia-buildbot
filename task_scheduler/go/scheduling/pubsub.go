package scheduling

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"

	"golang.org/x/net/context"

	"cloud.google.com/go/pubsub"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
)

const (
	PUBSUB_TOPIC_SWARMING_TASKS       = "swarming-tasks"
	PUBSUB_FULLY_QUALIFIED_TOPIC_TMPL = "projects/%s/topics/%s"
)

var (
	PUBSUB_TOPICS                    = []string{PUBSUB_TOPIC_SWARMING_TASKS}
	PUBSUB_TOPIC_SWARMING_TASKS_FULL = fmt.Sprintf(PUBSUB_FULLY_QUALIFIED_TOPIC_TMPL, common.PROJECT_ID, PUBSUB_TOPIC_SWARMING_TASKS)
)

// InitPubSub ensures that the pub/sub topics and subscriptions needed by the
// TaskScheduler exist.
func InitPubSub() error {
	ctx := context.Background()

	// Create a client.
	client, err := pubsub.NewClient(ctx, common.PROJECT_ID)
	if err != nil {
		return err
	}

	// Create topics and subscriptions if necessary.
	for _, topicName := range PUBSUB_TOPICS {
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
		subName := fmt.Sprintf("task_scheduler+%s", topicName)
		sub := client.Subscription(subName)
		exists, err = sub.Exists(ctx)
		if err != nil {
			return err
		}
		if !exists {
			c := &pubsub.PushConfig{
				Endpoint: fmt.Sprintf("https://task-scheduler.skia.org/pubsub/%s", topicName),
			}
			if _, err := client.CreateSubscription(ctx, subName, topic, 20*time.Second, c); err != nil {
				return err
			}
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

// RegisterPubSubServer adds handler to r that handle pub/sub push
// notifications.
func RegisterPubSubServer(s *TaskScheduler, r *mux.Router) {
	r.HandleFunc(fmt.Sprintf("/pubsub/%s", PUBSUB_TOPIC_SWARMING_TASKS), func(w http.ResponseWriter, r *http.Request) {
		var req PubSubRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			httputils.ReportError(w, r, err, "Failed to decode request body.")
			return
		}

		// TODO(borenet): Ensure that the auth token is correct.
		var t PubSubTaskMessage
		if err := json.Unmarshal(req.Message.Data, &t); err != nil {
			httputils.ReportError(w, r, err, "Failed to decode PubSubTaskMessage.")
			return
		}

		glog.Infof("Got task notification from Swarming: %s", t.SwarmingTaskId)
		/*if err := s.updateTaskFromSwarming(t.SwarmingTaskId); err != nil {
			httputils.ReportError(w, r, err, "Failed to process Swarming task.")
			return
		}*/
		w.WriteHeader(http.StatusOK)
	}).Methods(http.MethodPost)
}
