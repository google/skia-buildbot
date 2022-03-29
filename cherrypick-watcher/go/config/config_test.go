package config

import (
	"fmt"
	"io/ioutil"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestParseCfg(t *testing.T) {
	unittest.MediumTest(t)
	dir := testutils.TestDataDir(t)
	configFile := filepath.Join(dir, "test-config.json")
	cfgContents, err := ioutil.ReadFile(configFile)
	require.Nil(t, err)

	supportedBranchDeps, err := ParseCfg(cfgContents)
	require.Nil(t, err)
	require.Len(t, supportedBranchDeps, 1)
	bp := supportedBranchDeps[0]
	require.Equal(t, "skiabot-test", bp.SourceRepo)
	require.Equal(t, "c1", bp.SourceBranch)
	require.Equal(t, "skiabot-test", bp.TargetRepo)
	require.Equal(t, "c3", bp.TargetBranch)
	require.Equal(t, "Test custom message", bp.CustomMessage)
}

func TestParseCfgInvalid(t *testing.T) {
	unittest.SmallTest(t)

	supportedBranchDeps, err := ParseCfg([]byte("Hi Mom!"))
	require.Nil(t, supportedBranchDeps)
	require.Error(t, err)
	fmt.Println(err)
	require.Regexp(t, `Failed to parse the config file with contents:
Hi Mom!: invalid character 'H' looking for beginning of value`, err.Error())
}
