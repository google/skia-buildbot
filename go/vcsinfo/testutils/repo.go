package testutils

import (
	"io/ioutil"
	"os"
	"path/filepath"

	"go.skia.org/infra/go/sklog"
	go_testutils "go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util/zip"
)

// tempRepo is used to setup and teardown a temporary repo for unit testing.
type tempRepo struct {
	// Root of unzipped Git repo.
	Dir string
}

// TODO(stephana): Use GitBuilder instead of checking in a Git repo.
// See https://skia.googlesource.com/buildbot/+show/master/go/git/testutils/git_builder.go#233
// Note: This will require to refactor the tests in infra/go/vcsinfo/testutils.

// newTempRepoFrom returns a tempRepo instance based on the contents of the
// given zip file path. Unzips to a temporary directory which is stored in
// tempRepo.Dir.
func newTempRepoFrom(zipfile string) *tempRepo {
	tmpdir, err := ioutil.TempDir("", "skiaperf")
	if err != nil {
		sklog.Fatal("Failed to create testing Git repo:", err)
	}
	if err := zip.UnZip(tmpdir, zipfile); err != nil {
		sklog.Fatal("Failed to unzip testing Git repo:", err)
	}
	return &tempRepo{Dir: tmpdir}
}

// newTempRepo assumes the repo is called testrepo.zip, is in a directory
// called testdata under the directory of the unit test that is calling it
// and contains a single directory 'testrepo'.
func newTempRepo() *tempRepo {
	testDataDir, err := go_testutils.TestDataDir()
	if err != nil {
		sklog.Fatal("Failed to locate test data dir:", err)
	}
	ret := newTempRepoFrom(filepath.Join(testDataDir, "testrepo.zip"))
	ret.Dir = filepath.Join(ret.Dir, "testrepo")
	return ret
}

// Cleanup cleans up the temporary repo.
func (t *tempRepo) Cleanup() {
	if err := os.RemoveAll(t.Dir); err != nil {
		sklog.Fatal("Failed to clean up after test:", err)
	}
}
