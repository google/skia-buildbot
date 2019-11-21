package continuous_deploy

const (
	// TOPIC is the PubSub topic for continuous deploy messages.
	TOPIC = "skia-docker-builds"

	// The cloud project the above topic lives in.
	TOPIC_PROJECT_ID = "skia-public"
)

// The type that will stored as data when publishing messages to the above topic.
type BuildInfo struct {
	ImageName string `json:"image_name"`
	Tag       string `json:"tag"`
}
