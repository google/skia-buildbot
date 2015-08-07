package event

import (
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/util"
)

const (
	GLOBAL_GOOGLE_STORAGE = "global-google-storage-event"
)

func init() {
	// GLOBAL_GOOGLE_STORAGE even will be fired with an instance of GoogleStorageEventData
	eventbus.RegisterGlobalEvent(GLOBAL_GOOGLE_STORAGE, util.JSONCodec(&GoogleStorageEventData{}))
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
