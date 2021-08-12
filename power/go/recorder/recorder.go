package recorder

import (
	"go.skia.org/infra/go/sklog"
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

// gclRecorder implements the Recorder interface by storing the results
// to cloud logging.
// TODO(kjlubick): when we want longer term storage, or to reason about
// previous events, also write it to GCS.  For GCS, to better tolerate errors
// we may want to write the entire list of down bots to the file, but for GCL,
// in order to keep it easier for humans to read, we just want to log the deltas.
type gclRecorder struct {
}

func NewCloudLoggingRecorder() *gclRecorder {
	sklog.Info("Initializing after boot.  Next down bots may have already been failing.")
	return &gclRecorder{}
}

// NewlyDownBots fulfills the Recorder interface
func (r *gclRecorder) NewlyDownBots(bots []string) {
	for _, bot := range bots {
		sklog.Infof("New Down Bot: %q ", bot)
	}

}

// NewlyFixedBots fulfills the Recorder interface
func (r *gclRecorder) NewlyFixedBots(bots []string) {
	for _, bot := range bots {
		sklog.Infof("New Fixed Bot: %q ", bot)
	}
}

// PowercycledBots fulfills the Recorder interface
func (r *gclRecorder) PowercycledBots(user string, bots []string) {
	for _, bot := range bots {
		sklog.Infof("%s powercycled Bot: %q ", user, bot)
	}
}
