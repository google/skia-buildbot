package cli

import (
	"context"
	"path/filepath"
	"testing"

	"go.skia.org/infra/bazel/go/bazel"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/urfave/cli/v2"
)

func TestCheckCommand(t *testing.T) {
	got := CheckCommand()
	require.NotNil(t, got)
	assert.Equal(t, "check", got.Name)
}

func TestCheckCommand_action(t *testing.T) {
	path := filepath.Join(
		bazel.RunfilesDir(),
		"external/cabe_replay_data",
		// https://pinpoint-dot-chromeperf.appspot.com/job/16f46f1c260000
		"pinpoint_16f46f1c260000.zip")
	cCmd := checkCmd{
		commonCmd{
			pinpointJobID: "16f46f1c260000",
			recordToZip:   "",
			replayFromZip: path,
		},
	}

	ctx := context.Background()
	cliCtx := cli.NewContext(nil, nil, nil)
	cliCtx.Context = ctx
	err := cCmd.action(cliCtx)
	require.NoError(t, err)
}
