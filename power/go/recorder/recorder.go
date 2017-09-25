package recorder

import (
	"time"

	"go.skia.org/infra/go/sklog"
)

// The Recorder interface abstracts a way to record which bots the power-controller
// has noticed are down and which it has noticed are fixed. These should be recorded
// somewhere durable by implementations of this interface.
type Recorder interface {
	// A listener for bots that are freshly down. Implementations should record
	// this in some way.
	NewlyDownBots(bots []string)
	// A listener for bots that are freshly fixed. Implementations should record
	// this in some way.
	NewlyFixedBots(bots []string)
}

const CLOUD_LOGGING_GROUPING = "history"

// gclRecorder implements the Recorder interface by storing the results
// to cloud logging.
// TODO(kjlubick): when we want longer term storage, also write it to GCS
type gclRecorder struct {
}

func NewCloudLoggingRecorder() *gclRecorder {
	sklog.CustomLog(CLOUD_LOGGING_GROUPING, &sklog.LogPayload{
		Time:     time.Now(),
		Severity: sklog.INFO,
		Payload:  "Initializing after boot.  Next down bots may have already been failing.",
	})
	return &gclRecorder{}
}

// NewlyDownBots fulfills the Recorder interface
func (r *gclRecorder) NewlyDownBots(bots []string) {
	now := time.Now()
	for _, bot := range bots {
		sklog.CustomLog(CLOUD_LOGGING_GROUPING, &sklog.LogPayload{
			Time:     now,
			Severity: sklog.INFO,
			Payload:  "New Down Bot: " + bot,
		})
	}

}

// NewlyFixedBots fulfills the Recorder interface
func (r *gclRecorder) NewlyFixedBots(bots []string) {
	now := time.Now()
	for _, bot := range bots {
		sklog.CustomLog(CLOUD_LOGGING_GROUPING, &sklog.LogPayload{
			Time:     now,
			Severity: sklog.INFO,
			Payload:  "New Fixed Bot: " + bot,
		})
	}
}
