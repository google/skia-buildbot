package gitinfo

import (
	"path/filepath"
	"testing"
	"time"

	"skia.googlesource.com/buildbot.git/go/util"
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
			t.Errorf("For ts: %d Length returned is wrong: Got %s Want %d", tc.ts, got, want)
		}
	}
}

func TestLog(t *testing.T) {
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	r, err := NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false)
	if err != nil {
		t.Fatal(err)
	}

	got, err := r.Log("7a669cfa3f4cd3482a4fd03989f75efcc7595f7f", "")
	if err != nil {
		t.Fatal(err)
	}
	want := `commit 7a669cfa3f4cd3482a4fd03989f75efcc7595f7f
Author: Joe Gregorio <jcgregorio@google.com>
Date:   Wed Jul 30 08:00:42 2014 -0400

    First "checkin"

    With quotes.

README.txt
`

	if got != want {
		t.Errorf("Log failed: \nGot  %q \nWant %q", got, want)
	}

	got, err = r.Log("7a669cfa3f4cd3482a4fd03989f75efcc7595f7f", "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18")
	if err != nil {
		t.Fatal(err)
	}
	want = `commit 8652a6df7dc8a7e6addee49f6ed3c2308e36bd18
Author: Joe Gregorio <jcgregorio@google.com>
Date:   Wed Jul 30 08:01:55 2014 -0400

    Added code. No body.

README.txt
hello.go
`

	if got != want {
		t.Errorf("Log failed: \nGot  %q \nWant %q", got, want)
	}

}

func TestShortList(t *testing.T) {
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	r, err := NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false)
	if err != nil {
		t.Fatal(err)
	}

	l, err := r.ShortList("8652a6df7dc8a7e6addee49f6ed3c2308e36bd18", "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18")
	if err != nil {
		t.Fatal(err)
	}

	if got, want := len(l.Commits), 0; got != want {
		t.Fatalf("Wrong number of zero results: Got %v Want %v", got, want)
	}

	c, err := r.ShortList("7a669cfa3f4cd3482a4fd03989f75efcc7595f7f", "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18")
	if err != nil {
		t.Fatal(err)
	}

	expected := []struct {
		Hash    string
		Author  string
		Subject string
	}{
		{
			Hash:    "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
			Author:  "Joe Gregorio",
			Subject: "Added code. No body.",
		},
	}
	if got, want := len(c.Commits), len(expected); got != want {
		t.Fatalf("Wrong number of results: Got %v Want %v", got, want)
	}
	for i, o := range c.Commits {
		if got, want := o.Hash, expected[i].Hash; got != want {
			t.Errorf("Wrong hash: Got %v Want %v", got, want)
		}
		if got, want := o.Author, expected[i].Author; got != want {
			t.Errorf("Wrong author: Got %v Want %v", got, want)
		}
		if got, want := o.Subject, expected[i].Subject; got != want {
			t.Errorf("Wrong subject: Got %v Want %v", got, want)
		}
	}
}

func TestTileAddressFromHash(t *testing.T) {
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	r, err := NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false)
	if err != nil {
		t.Fatal(err)
	}

	// The two commits in the repo have timestamps of:
	// 1406721715 and 1406721642.
	testCases := []struct {
		hash   string
		start  time.Time
		num    int
		offset int
	}{
		{
			hash:   "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
			start:  time.Unix(1406721642, 0),
			num:    0,
			offset: 1,
		},
		{
			hash:   "8652a6df7dc8a7e6addee49f6ed3c2308e36bd18",
			start:  time.Unix(1406721643, 0),
			num:    0,
			offset: 0,
		},
	}

	for _, tc := range testCases {
		n, o, err := r.TileAddressFromHash(tc.hash, tc.start)
		if err != nil {
			t.Fatal(err)
		}
		if n != tc.num || o != tc.offset {
			t.Errorf("Address unexpected: got (%d, %d), want (%d, %d).", tc.num, tc.offset, n, o)
		}
	}
}

func TestNumCommits(t *testing.T) {
	tr := util.NewTempRepo()
	defer tr.Cleanup()

	r, err := NewGitInfo(filepath.Join(tr.Dir, "testrepo"), false)
	if err != nil {
		t.Fatal(err)
	}

	if got, want := r.NumCommits(), 2; got != want {
		t.Errorf("NumCommit wrong number: Got %v Want %v", got, want)
	}
}
