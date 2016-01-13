package event

import (
	"crypto/md5"
	"fmt"
	"strings"

	"github.com/skia-dev/glog"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/util"
)

const (
	GLOBAL_BUILDBOT       = "global-buildbot-event"
	GLOBAL_GOOGLE_STORAGE = "global-google-storage-event"
)

func init() {
	// GLOBAL_GOOGLE_STORAGE even will be fired with an instance of GoogleStorageEventData
	eventbus.RegisterGlobalEvent(GLOBAL_GOOGLE_STORAGE, util.JSONCodec(&GoogleStorageEventData{}))
	eventbus.RegisterGlobalEvent(GLOBAL_BUILDBOT, util.JSONCodec(&BuildbotEventData{}))
}

func StorageEvent(bucket, prefix string) string {
	// Generate a unique topic name. This is also necessary because bucket and prefix values
	// can contain many more different characters than event names.
	subTopic := fmt.Sprintf("%s-%x", GLOBAL_GOOGLE_STORAGE, md5.Sum([]byte(bucket+"/"+prefix)))
	eventbus.RegisterSubTopic(GLOBAL_GOOGLE_STORAGE, subTopic, func(eventData interface{}) bool {
		gsEvent, ok := eventData.(*GoogleStorageEventData)
		if !ok {
			glog.Errorf("Received data that was not an instance of GoogleStorageEventData.")
			return false
		}

		return (gsEvent.Bucket == bucket) && strings.HasPrefix(gsEvent.Name, prefix)
	})
	return subTopic
}

type GoogleStorageEventData struct {
	Kind           string            `json:"kind"`
	Id             string            `json:""`
	SelfLink       string            `json:"selfLink"`
	Name           string            `json:"name"`
	Bucket         string            `json:"bucket"`
	Generation     string            `json:"generation"`
	Metageneration string            `json:"metageneration"`
	ContentType    string            `json:"contentType"`
	Updated        string            `json:"updated"`
	TimeDeleted    string            `json:"timeDeleted"`
	StorageClass   string            `json:"storageClass"`
	Size           string            `json:"size"`
	Md5Hash        string            `json:"md5hash"`
	MediaLink      string            `json:"mediaLink"`
	Owner          map[string]string `json:"owner"`
	Crc32C         string            `json:"crc32c"`
	ETag           string            `json:"etag"`
}

// BuildbotEventType registers a subtopic and returns a subtopic name for the
// given buildbot event type.
func BuildbotEventType(eventType string) string {
	subTopic := fmt.Sprintf("%s-%s", GLOBAL_BUILDBOT, eventType)
	eventbus.RegisterSubTopic(GLOBAL_BUILDBOT, subTopic, func(eventData interface{}) bool {
		e, ok := eventData.(*BuildbotEventData)
		if !ok {
			glog.Errorf("Received data that was not an instance of BuildbotEventData.")
			return false
		}
		return e.Event == eventType
	})
	return subTopic
}

type BuildbotEventData struct {
	Id      int64                  `json:"id"`
	Event   string                 `json:"event"`
	Payload map[string]interface{} `json:"payload"`
	Project string                 `json:"project"`
}
