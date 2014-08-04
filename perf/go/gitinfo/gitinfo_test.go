package gitinfo

import (
	"path/filepath"
	"testing"
	"time"

	"skia.googlesource.com/buildbot.git/perf/go/util"
)

func TestDisplay(t *testing.T) {
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	r, err := NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false)
	if err != nil {
		t.Fatal(err)
	}
	testCases := []struct {
		hash    string
		author  string
		subject string
	}{
		{
			hash:    "7a669cfa3f4cd3482a4fd03989f75efcc7595f7f",
			author:  "Joe Gregorio (jcgregorio@google.com)",
			subject: "First \"checkin\"",
		},
		{
			hash:    "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
			author:  "Joe Gregorio (jcgregorio@google.com)",
			subject: "Added code. No body.",
		},
	}
	for _, tc := range testCases {
		author, subject, _, err := r.Details(tc.hash)
		if err != nil {
			t.Fatal(err)
		}
		if got, want := author, tc.author; got != want {
			t.Errorf("Details author mismatch: Got %q, Want %q", got, want)
		}
		if got, want := subject, tc.subject; got != want {
			t.Errorf("Details subject mismatch: Got %q, Want %q", got, want)
		}
	}
}

func TestFrom(t *testing.T) {
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	r, err := NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false)
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
