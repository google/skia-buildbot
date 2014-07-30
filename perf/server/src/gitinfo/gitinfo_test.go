package gitinfo

import (
	"archive/zip"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"
)

var (
	tmpdir string
)

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

func setup() {
	var err error
	if tmpdir, err = ioutil.TempDir("", "skiaperf"); err != nil {
		log.Fatalln("Failed to create testing Git repo:", err)
	}
	_, filename, _, _ := runtime.Caller(0)
	if err = unzip(filepath.Join(filepath.Dir(filename), "testdata", "testrepo.zip"), tmpdir); err != nil {
		log.Fatalln("Failed to unzip testing Git repo:", err)
	}
}

func teardown() {
	if err := os.RemoveAll(tmpdir); err != nil {
		log.Fatalln("Failed to clean up after test:", err)
	}
}

func TestDisplay(t *testing.T) {
	setup()
	defer teardown()

	r, err := NewGitInfo(filepath.Join(tmpdir, "testrepo"), false)
	if err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		hash    string
		subject string
		body    string
	}{
		{
			hash:    "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f",
			subject: "First \"checkin\"",
			body:    "With quotes.\n",
		},
		{
			hash:    "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
			subject: "Added code. No body.",
			body:    "",
		},
	}
	for _, tc := range testCases {
		subject, body, err := r.Details(tc.hash)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := subject, tc.subject; got != want {
			t.Errorf("Details subject mismatch: Got %q, Want %q", got, want)
		}
		if got, want := body, tc.body; got != want {
			t.Errorf("Details subject mismatch: Got %q, Want %q", got, want)
		}
	}
}

func TestFrom(t *testing.T) {
	setup()
	defer teardown()

	r, err := NewGitInfo(filepath.Join(tmpdir, "testrepo"), false)
	if err != nil {
		t.Fatal(err)
	}

	// The two commits in the repo have timestamps of:
	// 1406721715 and 1406721642.
	testCases := []struct {
		ts     int64
		length int
	}{
		{
			ts:     1406721715,
			length: 0,
		},
		{
			ts:     1406721714,
			length: 1,
		},
		{
			ts:     1406721642,
			length: 1,
		},
		{
			ts:     1406721641,
			length: 2,
		},
	}
	for _, tc := range testCases {
		hashes := r.From(time.Unix(tc.ts, 0))
		if got, want := len(hashes), tc.length; got != want {
			t.Error("For ts: %d Length returned is wrong: Got %s Want %d", tc.ts, got, want)
		}
	}
}
