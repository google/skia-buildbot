package util

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"runtime"

	"github.com/skia-dev/glog"
)

// unzip unzips the file given in src into the 'dest' directory.
func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer Close(r)

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer Close(rc)

		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			if err := os.MkdirAll(path, f.Mode()); err != nil {
				return err
			}
		} else {
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer Close(f)

			_, err = io.Copy(f, rc)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// TempRepo is used to setup and teardown a temporary repo for unit testing.
type TempRepo struct {
	// Root of unzipped Git repo.
	Dir string
}

// NewTempRepo assumes the repo is called testrepo.zip and is in a directory
// called testdata under the directory of the unit test that is calling it.
//
// The directory that was created is stored in TempRepo Path.
func NewTempRepo() *TempRepo {
	tmpdir, err := ioutil.TempDir("", "skiaperf")
	if err != nil {
		glog.Fatalln("Failed to create testing Git repo:", err)
	}
	_, filename, _, _ := runtime.Caller(1)
	if err := unzip(filepath.Join(filepath.Dir(filename), "testdata", "testrepo.zip"), tmpdir); err != nil {
		glog.Fatalln("Failed to unzip testing Git repo:", err)
	}
	return &TempRepo{Dir: tmpdir}
}

// Cleanup cleans up the temporary repo.
func (t *TempRepo) Cleanup() {
	if err := os.RemoveAll(t.Dir); err != nil {
		glog.Fatalln("Failed to clean up after test:", err)
	}
}
