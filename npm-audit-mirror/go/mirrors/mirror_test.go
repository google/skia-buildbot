// Copyright 2022 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package mirrors

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/executil"
	"go.skia.org/infra/go/testutils"
)

var (
	// This config path is only used for asserting the start command. It does
	// not have to exist and will not be created.
	testVerdaccioConfigPath = "/tmp/test-config-path1"
)

func TestStartVerdaccioMirror_VerdaccioStartCommandIsCalled_Success(t *testing.T) {

	// Mock the executil call.
	ctx := executil.FakeTestsContext("Test_FakeExe_Verdaccio_Start_Cmd")

	m := &VerdaccioMirror{
		verdaccioConfigPath: testVerdaccioConfigPath,
	}
	m.startVerdaccioMirror(ctx, 1111, nil)
	require.Equal(t, executil.FakeCommandsReturned(ctx), 1)
}

func TestDownloadedPackageTarballs(t *testing.T) {

	m := &VerdaccioMirror{downloadedPackageTarballs: map[string]interface{}{}}
	require.False(t, m.IsPackageTarballDownloaded("pkg1.tgz"))
	m.AddToDownloadedPackageTarballs("pkg1.tgz")
	require.True(t, m.IsPackageTarballDownloaded("pkg1.tgz"))
}

func TestGetDownloadedPackageNames(t *testing.T) {
	storageDir := testutils.TestDataDir(t)

	m := &VerdaccioMirror{verdaccioStorageDir: storageDir}
	packages, err := m.GetDownloadedPackageNames()
	require.NoError(t, err)
	require.Equal(t, []string{"pkg1", "pkg2"}, packages)
}

func TestGetTarballsInMirrorStorage(t *testing.T) {
	storageDir := testutils.TestDataDir(t)

	tarballs, err := GetTarballsInMirrorStorage(storageDir)
	require.NoError(t, err)
	_, pkg1ok := tarballs["pkg1.tgz"]
	require.True(t, pkg1ok)
	_, pkg2ok := tarballs["pkg2.tgz"]
	require.True(t, pkg2ok)
	_, pkg3ok := tarballs["pkg3.tgz"]
	require.False(t, pkg3ok)
}

func TestMigrateVerdaccioDBSecret_MissingFile_NoOp(t *testing.T) {
	require.NoError(t, migrateVerdaccioDBSecret(t.TempDir()))
}

func TestMigrateVerdaccioDBSecret_Valid32CharSecret_Unchanged(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, verdaccioDBFileName)
	validSecret := strings.Repeat("a", verdaccioSecretKeyLength)
	orig := fmt.Sprintf(`{"list":["pkg1"],"secret":%q}`, validSecret)
	require.NoError(t, os.WriteFile(dbPath, []byte(orig), 0644))

	require.NoError(t, migrateVerdaccioDBSecret(dir))

	got, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	require.Equal(t, orig, string(got))
}

func TestMigrateVerdaccioDBSecret_Legacy64CharSecret_ClearedAndPreservesList(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, verdaccioDBFileName)
	legacySecret := strings.Repeat("b", 64)
	orig := fmt.Sprintf(`{"list":["pkg1","pkg2"],"secret":%q}`, legacySecret)
	require.NoError(t, os.WriteFile(dbPath, []byte(orig), 0644))

	require.NoError(t, migrateVerdaccioDBSecret(dir))

	gotBytes, err := os.ReadFile(dbPath)
	require.NoError(t, err)
	var db map[string]any
	require.NoError(t, json.Unmarshal(gotBytes, &db))
	require.Equal(t, "", db["secret"])
	require.Equal(t, []any{"pkg1", "pkg2"}, db["list"])
}

func TestMigrateVerdaccioDBSecret_InvalidJSON_ReturnsError(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, verdaccioDBFileName)
	require.NoError(t, os.WriteFile(dbPath, []byte("{not-json"), 0644))

	require.Error(t, migrateVerdaccioDBSecret(dir))
}

// This is not a real test, but a fake implementation of the executable in question.
// By convention, we prefix these with FakeExe to make that clear.
func Test_FakeExe_Verdaccio_Start_Cmd(t *testing.T) {
	// Since this is a normal go test, it will get run on the usual test suite. We check for the
	// special environment variable and if it is not set, we do nothing.
	if !executil.IsCallingFakeCommand() {
		return
	}

	// Check the input arguments to make sure they were as expected.
	args := executil.OriginalArgs()
	fmt.Printf("")
	require.Equal(t, []string{"verdaccio", fmt.Sprintf("--config=%s", testVerdaccioConfigPath), "--listen=1111", "--audit-level=high"}, args)

	os.Exit(0) // exit 0 prevents golang from outputting test stuff like "=== RUN", "---Fail".
}
