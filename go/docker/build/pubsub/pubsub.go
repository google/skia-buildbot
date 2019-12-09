package pubsub

import (
	"fmt"
)

const (
	// TOPIC is the PubSub topic for docker image push messages.
	TOPIC = "skia-docker-builds"

	// The cloud project the above topic lives in.
	TOPIC_PROJECT_ID = "skia-public"
)

// The type that will stored as data when publishing messages to the above topic.
type BuildInfo struct {
	ImageName string `json:"image_name"`
	Tag       string `json:"tag"`
	Repo      string `json:"repo"`
}

func (b BuildInfo) String() string {
	return fmt.Sprintf("%s:%s", b.ImageName, b.Tag)
}
