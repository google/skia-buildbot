package recorder

import (
	"time"

	"go.skia.org/infra/go/sklog/glog_and_cloud"
)

// Recorder records which bots the power-controller has noticed are down and
// which it has noticed are fixed.
type Recorder interface {
	// NewlyDownBots records a set of bot names that are freshly down. This
	// is intended to act like a listener, thus clients should handle any errors.
	NewlyDownBots(bots []string)
	// NewlyFixedBots records a set of bot names that are freshly fixed. This
	// is intended to act like a listener, thus clients should handle any errors.
	NewlyFixedBots(bots []string)
	// PowercycledBots records a set of bot names that were powercycled by the
	// specified user.
	PowercycledBots(user string, bots []string)
}

const CLOUD_LOGGING_GROUPING = "history"

// gclRecorder implements the Recorder interface by storing the results
// to cloud logging.
// TODO(kjlubick): when we want longer term storage, or to reason about
// previous events, also write it to GCS.  For GCS, to better tolerate errors
// we may want to write the entire list of down bots to the file, but for GCL,
// in order to keep it easier for humans to read, we just want to log the deltas.
type gclRecorder struct {
}

func NewCloudLoggingRecorder() *gclRecorder {
	glog_and_cloud.CustomLog(CLOUD_LOGGING_GROUPING, &glog_and_cloud.LogPayload{
		Time:     time.Now(),
		Severity: glog_and_cloud.INFO,
		Payload:  "Initializing after boot.  Next down bots may have already been failing.",
	})
	return &gclRecorder{}
}

// NewlyDownBots fulfills the Recorder interface
func (r *gclRecorder) NewlyDownBots(bots []string) {
	now := time.Now()
	for _, bot := range bots {
		glog_and_cloud.CustomLog(CLOUD_LOGGING_GROUPING, &glog_and_cloud.LogPayload{
			Time:     now,
			Severity: glog_and_cloud.INFO,
			Payload:  "New Down Bot: " + bot,
		})
	}

}

// NewlyFixedBots fulfills the Recorder interface
func (r *gclRecorder) NewlyFixedBots(bots []string) {
	now := time.Now()
	for _, bot := range bots {
		glog_and_cloud.CustomLog(CLOUD_LOGGING_GROUPING, &glog_and_cloud.LogPayload{
			Time:     now,
			Severity: glog_and_cloud.INFO,
			Payload:  "New Fixed Bot: " + bot,
		})
	}
}

// PowercycledBots fulfills the Recorder interface
func (r *gclRecorder) PowercycledBots(user string, bots []string) {
	now := time.Now()
	for _, bot := range bots {
		glog_and_cloud.CustomLog(CLOUD_LOGGING_GROUPING, &glog_and_cloud.LogPayload{
			Time:     now,
			Severity: glog_and_cloud.INFO,
			Payload:  user + " powercycled Bot: " + bot,
		})
	}
}
