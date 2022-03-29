package config

import (
	"fmt"
	"io/ioutil"
	"os"
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

	supportedBranchDeps, err := ParseCfg(configFile)
	require.Nil(t, err)
	require.Len(t, supportedBranchDeps, 1)
	bp := supportedBranchDeps[0]
	require.Equal(t, "skiabot-test", bp.SourceRepo)
	require.Equal(t, "c1", bp.SourceBranch)
	require.Equal(t, "skiabot-test", bp.TargetRepo)
	require.Equal(t, "c3", bp.TargetBranch)
	require.Equal(t, "Test custom message", bp.CustomMessage)
}

func TestParseCfgDoesntExist(t *testing.T) {
	unittest.MediumTest(t)
	dir := t.TempDir()
	configFile := filepath.Join(dir, "nonexistent-config.json")

	supportedBranchDeps, err := ParseCfg(configFile)
	require.Nil(t, supportedBranchDeps)
	require.Error(t, err)
	require.Regexp(t, `Could not read the config file .*/nonexistent-config.json`, err.Error())
}

func TestParseCfgInvalid(t *testing.T) {
	unittest.MediumTest(t)
	dir := t.TempDir()
	configFile := filepath.Join(dir, "invalid-config.json")
	require.NoError(t, ioutil.WriteFile(configFile, []byte("Hi Mom!"), os.ModePerm))

	supportedBranchDeps, err := ParseCfg(configFile)
	require.Nil(t, supportedBranchDeps)
	require.Error(t, err)
	fmt.Println(err)
	require.Regexp(t, `Failed to parse the config file with contents:
Hi Mom!: invalid character 'H' looking for beginning of value`, err.Error())
}
