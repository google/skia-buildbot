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

func TestReadCASCommand(t *testing.T) {
	got := ReadCASCommand()
	require.NotNil(t, got)
	assert.Equal(t, "readcas", got.Name)
}

func TestReadCASCommand_action(t *testing.T) {
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
	casCmd := readCASCmd{
		commonCmd:   cCmd,
		rootDigest:  "587d9372661b9506c3df2ef384532a1215901beeda01a3002772be4ead97d480/178",
		casInstance: "projects/chrome-swarming/instances/default_instance",
	}

	ctx := context.Background()
	cliCtx := cli.NewContext(nil, nil, nil)
	cliCtx.Context = ctx

	err := casCmd.action(cliCtx)
	require.NoError(t, err)
}
