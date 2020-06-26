package testutils

import (
	"io/ioutil"
	"path/filepath"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/util/zip"
)

// tempRepo is used to setup and teardown a temporary repo for unit testing.
type tempRepo struct {
	// Root of unzipped Git repo.
	Dir string
	t   sktest.TestingT
}

// TODO(stephana): Use GitBuilder instead of checking in a Git repo.
// See https://skia.googlesource.com/buildbot/+show/master/go/git/testutils/git_builder.go#233
// Note: This will require to refactor the tests in infra/go/vcsinfo/testutils.

// newTempRepoFrom returns a tempRepo instance based on the contents of the
// given zip file path. Unzips to a temporary directory which is stored in
// tempRepo.Dir.
func newTempRepoFrom(t sktest.TestingT, zipfile string) *tempRepo {
	tmpdir, err := ioutil.TempDir("", "skiaperf")
	require.NoError(t, err)
	err = zip.UnZip(tmpdir, zipfile)
	require.NoError(t, err)
	return &tempRepo{
		Dir: tmpdir,
		t:   t,
	}
}

// newTempRepo assumes the repo is called testrepo.zip, is in a directory
// called testdata under the directory of the unit test that is calling it
// and contains a single directory 'testrepo'.
func newTempRepo(t sktest.TestingT) *tempRepo {
	ret := newTempRepoFrom(t, filepath.Join(testutils.TestDataDir(t), "testrepo.zip"))
	ret.Dir = filepath.Join(ret.Dir, "testrepo")
	return ret
}

// Cleanup cleans up the temporary repo.
func (t *tempRepo) Cleanup() {
	testutils.RemoveAll(t.t, t.Dir)
}
