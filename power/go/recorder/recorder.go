package recorder

import (
	"time"

	"go.skia.org/infra/go/sklog"
)

type Recorder interface {
	// A listener for bots that are freshly down
	NewlyDownBots(bots []string)
	// A listener for bots that are freshly fixed
	NewlyFixedBots(bots []string)
}

type gcsRecorder struct {
}

func NewGCSAndCloudLoggingRecorder() *gcsRecorder {
	sklog.CustomLog("history", &sklog.LogPayload{
		Time:     time.Now(),
		Severity: sklog.INFO,
		Payload:  "Initializing after boot.  Next down bots may have already been failing.",
	})
	return &gcsRecorder{}
}

func (r *gcsRecorder) NewlyDownBots(bots []string) {
	now := time.Now()
	for _, bot := range bots {
		sklog.CustomLog("history", &sklog.LogPayload{
			Time:     now,
			Severity: sklog.INFO,
			Payload:  "New Down Bot: " + bot,
		})
	}

}

func (r *gcsRecorder) NewlyFixedBots(bots []string) {
	now := time.Now()
	for _, bot := range bots {
		sklog.CustomLog("history", &sklog.LogPayload{
			Time:     now,
			Severity: sklog.INFO,
			Payload:  "New Fixed Bot: " + bot,
		})
	}
}
