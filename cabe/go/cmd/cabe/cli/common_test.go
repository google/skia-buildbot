package cli

import (
	"context"
	"path/filepath"
	"testing"

	"go.skia.org/infra/bazel/go/bazel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_commonCmd_flags(t *testing.T) {
	for _, test := range []struct {
		name       string
		cCmd       commonCmd
		wantLength int
	}{
		{
			name:       "empty",
			wantLength: 3,
		}, {
			name: "complete input",
			cCmd: commonCmd{
				pinpointJobID: "testPinpointJobID",
				recordToZip:   "testRecordToZip",
				replayFromZip: "testReplayFromZip",
			},
			wantLength: 3,
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			cmd := &commonCmd{
				pinpointJobID: test.cCmd.pinpointJobID,
				recordToZip:   test.cCmd.recordToZip,
				replayFromZip: test.cCmd.replayFromZip,
			}
			got := cmd.flags()
			assert.Equal(t, test.wantLength, len(got))
		})
	}
}

func Test_commonCmd_readCASResultFromRBEAPI(t *testing.T) {
	path := filepath.Join(
		bazel.RunfilesDir(),
		"external/cabe_replay_data",
		// https://pinpoint-dot-chromeperf.appspot.com/job/16f46f1c260000
		"pinpoint_16f46f1c260000.zip")
	cCmd := commonCmd{
		pinpointJobID: "16f46f1c260000",
		recordToZip:   "",
		replayFromZip: path,
	}

	ctx := context.Background()
	err := cCmd.dialBackends(ctx)
	require.NoError(t, err)
}
