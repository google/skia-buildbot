package replaybackends

import (
	"path/filepath"
	"testing"

	"go.skia.org/infra/bazel/go/bazel"
)

// https://pinpoint-dot-chromeperf.appspot.com/job/14a7fea47a0000
// chosen because it contains task failures.
const pinpointReplayFile = "pinpoint_14a7fea47a0000.zip"

func TestFromZipFile(t *testing.T) {
	// ctx := context.Background()
	replayers := FromZipFile(
		filepath.Join(
			bazel.RunfilesDir(),
			"external/cabe_replay_data/",
			pinpointReplayFile,
		),
		"fake benchmark name",
	)

	if replayers == nil {
		t.Errorf("replayers was nil when it should not be")
		return
	}

	if replayers.CASResultReader == nil {
		t.Errorf("CASResultReader was nil when it should not be")
	}

}
