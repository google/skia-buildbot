package event

import (
	"crypto/md5"
	"fmt"
	"strings"
	"sync"

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

// BotBilter is a container for chainable filters for BuildBotEvents.
type BotFilter struct {
	mutex      sync.RWMutex
	ids        []string
	predicates []func(e *BuildbotEventData) bool
}

// BotEventFilter returns a new chainable filter for bot events.
func BotEventFilter() *BotFilter {
	return &BotFilter{
		ids:        []string{},
		predicates: []func(*BuildbotEventData) bool{},
	}
}

// EventType allows to specified a set of event types that should be
// include in the buildbot events that are delivered via the subscription
// to a subtopic of all buildbot events. If any of the arguments is
// the empty string, the filter will be ignored.
func (b *BotFilter) EventType(eTypes ...string) *BotFilter {
	lookup := util.StringSet(eTypes)
	if lookup[""] {
		return b
	}
	return b.append(eTypes, func(ev *BuildbotEventData) bool {
		return lookup[ev.Event]
	})
}

// EventType allows to specified a set of step names that should be
// include in the buildbot events that are delivered via the subscription
// to a subtopic of all buildbot events. If any of the arguments is
// the empty string, the filter will be ignored.
func (b *BotFilter) StepName(stepNames ...string) *BotFilter {
	lookup := util.StringSet(stepNames)
	if lookup[""] {
		return b
	}
	return b.append(stepNames, func(ev *BuildbotEventData) bool {
		return lookup[getStepName(ev)]
	})
}

func (b *BotFilter) append(ids []string, fn func(*BuildbotEventData) bool) *BotFilter {
	b.mutex.Lock()
	defer b.mutex.Unlock()
	b.ids = append(b.ids, ids...)
	b.predicates = append(b.predicates, fn)
	return b
}

func (b *BotFilter) filter(ev *BuildbotEventData) bool {
	b.mutex.RLock()
	defer b.mutex.RUnlock()
	for _, fn := range b.predicates {
		if !fn(ev) {
			return false
		}
	}
	return true
}

// BuildbotEvents registers a subtopic and returns a subtopic name for the
// given buildbot event type. If the argument is empty or now filters were
// added to the BotFilter instance, all buildbot events will be delievered.
func BuildbotEvents(botFilter *BotFilter) string {
	botFilter.mutex.Lock()
	defer botFilter.mutex.Unlock()

	if (botFilter == nil) || (len(botFilter.ids) == 0) {
		return GLOBAL_BUILDBOT
	}

	// Make a copy of the predicates and genrate the topic name.
	ids := append([]string{GLOBAL_BUILDBOT}, botFilter.ids...)
	subTopic := strings.Join(ids, "-")

	eventbus.RegisterSubTopic(GLOBAL_BUILDBOT, subTopic, func(eventData interface{}) bool {
		e, ok := eventData.(*BuildbotEventData)
		if !ok {
			glog.Errorf("Received data that was not an instance of BuildbotEventData.")
			return false
		}
		return botFilter.filter(e)
	})
	return subTopic
}

type BuildbotEventData struct {
	Id      int64                  `json:"id"`
	Event   string                 `json:"event"`
	Payload map[string]interface{} `json:"payload"`
	Project string                 `json:"project"`
}

// getStepName robustly extracts the step name if one is present in Payload.
func getStepName(e *BuildbotEventData) string {
	if step, ok := e.Payload["step"]; ok {
		if name, ok := step.(map[string]interface{})["name"]; ok {
			return name.(string)
		}
	}
	return ""
}
