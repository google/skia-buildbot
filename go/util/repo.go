package util

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
)

// unzip unzips the file given in src into the 'dest' directory.
func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return err
		}
		defer rc.Close()

		path := filepath.Join(dest, f.Name)
		if f.FileInfo().IsDir() {
			os.MkdirAll(path, f.Mode())
		} else {
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
			if err != nil {
				return err
			}
			defer f.Close()

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
		log.Fatalln("Failed to create testing Git repo:", err)
	}
	_, filename, _, _ := runtime.Caller(1)
	if err := unzip(filepath.Join(filepath.Dir(filename), "testdata", "testrepo.zip"), tmpdir); err != nil {
		log.Fatalln("Failed to unzip testing Git repo:", err)
	}
	return &TempRepo{Dir: tmpdir}
}

// Cleanup cleans up the temporary repo.
func (t *TempRepo) Cleanup() {
	if err := os.RemoveAll(t.Dir); err != nil {
		log.Fatalln("Failed to clean up after test:", err)
	}
}
