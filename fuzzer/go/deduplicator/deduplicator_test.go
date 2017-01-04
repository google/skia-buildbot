package deduplicator

import (
	"fmt"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/go/testutils"
)

func TestLocalSimpleDeduplication(t *testing.T) {
	testutils.SmallTest(t)
	d := NewLocalDeduplicator()
	r1 := data.MockReport("skpicture", "aaaa")
	r2 := data.MockReport("skpicture", "bbbb")
	// mock report ffff and aaaa are the same, except for the name.
	r3 := data.MockReport("skpicture", "ffff")
	// mock report jjjj and aaaa are the same, except for the name and architecture.
	r4 := data.MockReport("skpicture", "jjjj")
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	if !d.IsUnique(r2) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r2)
	}
	if d.IsUnique(r1) {
		t.Errorf("Should not have said %#v was unique, it just saw it.", r1)
	}
	if d.IsUnique(r3) {
		t.Errorf("Should not have said %#v was unique, it just saw something like it.", r3)
	}
	if !d.IsUnique(r4) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r4)
	}
}

func TestLocalUnknownStacktraces(t *testing.T) {
	testutils.SmallTest(t)
	d := NewLocalDeduplicator()
	// mock report ee has no stacktrace for either.  It should not be considered a duplicate, ever.
	r1 := data.MockReport("skpicture", "eeee")
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	if !d.IsUnique(r1) {
		t.Errorf("Should not have said %#v was not unique, unknown stacktraces don't count.", r1)
	}
}

func TestKey(t *testing.T) {
	testutils.SmallTest(t)
	// r1 is a report with 6 and 7 stacktrace frames for Debug/Release
	r1 := makeReport()
	r1.DebugStackTrace.Frames = append(r1.DebugStackTrace.Frames, data.StackTraceFrame{})
	r1.ReleaseStackTrace.Frames = append(r1.DebugStackTrace.Frames, data.StackTraceFrame{})

	k1 := key(r1)
	k2 := key(r1)
	if k1 != k2 {
		t.Errorf("Keys should be deterministic\n%s != %s", k1, k2)
	}
	if n := len(r1.DebugStackTrace.Frames); n != _MAX_STACKTRACE_LINES+1 {
		t.Errorf("key() should not have changed the report - it now has %d frames instead of 6", n)
		t.Errorf(r1.DebugStackTrace.String())
	}
	if n := len(r1.ReleaseStackTrace.Frames); n != _MAX_STACKTRACE_LINES+2 {
		t.Errorf("key() should not have changed the report - it now has %d frames instead of 7", n)
		t.Errorf(r1.DebugStackTrace.String())
	}
	if frame := r1.DebugStackTrace.Frames[0]; frame.LineNumber == 0 {
		t.Errorf("key() should not have changed the report - it now has the wrong line number at index 0: %s", r1.DebugStackTrace.String())
	}
	if frame := r1.ReleaseStackTrace.Frames[0]; frame.LineNumber == 0 {
		t.Errorf("key() should not have changed the report - it now has the wrong line number at index 0: %s", r1.ReleaseStackTrace.String())
	}
}

func TestLocalLinesOfStacktrace(t *testing.T) {
	testutils.SmallTest(t)
	d := NewLocalDeduplicator()
	r1 := makeReport()
	r2 := makeReport()
	r2.DebugStackTrace.Frames = append(r2.DebugStackTrace.Frames, data.StackTraceFrame{})
	r3 := makeReport()
	r3.ReleaseStackTrace.Frames = append(r3.ReleaseStackTrace.Frames, data.StackTraceFrame{})
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	if d.IsUnique(r2) {
		t.Errorf("Should not have said %#v was unique, it just saw something with the same top %d stacktraces.", r2, _MAX_STACKTRACE_LINES)
		t.Errorf("Debug stacktraces: \n%s\n\n%s", r1.DebugStackTrace.String(), r2.DebugStackTrace.String())
		t.Errorf("Release stacktraces: \n%s\n\n%s", r1.ReleaseStackTrace.String(), r2.ReleaseStackTrace.String())
	}
	if d.IsUnique(r3) {
		t.Errorf("Should not have said %#v was unique, it just saw something with the same top %d stacktraces.", r3, _MAX_STACKTRACE_LINES)
		t.Errorf("Debug stacktraces: \n%s\n\n%s", r1.DebugStackTrace.String(), r3.DebugStackTrace.String())
		t.Errorf("Release stacktraces: \n%s\n\n%s", r1.ReleaseStackTrace.String(), r3.ReleaseStackTrace.String())
	}
}

func TestLocalLineNumbers(t *testing.T) {
	testutils.SmallTest(t)
	d := NewLocalDeduplicator()
	r1 := makeReport()
	r2 := makeReport()
	r2.DebugStackTrace.Frames[0].LineNumber = 9999
	r3 := makeReport()
	r3.ReleaseStackTrace.Frames[0].LineNumber = 9999
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	if d.IsUnique(r2) {
		t.Errorf("Should not have said %#v was unique, it just saw something with the same top %d stacktraces, not counting line numbers.", r2, _MAX_STACKTRACE_LINES)
		t.Errorf("Debug stacktraces: \n%s\n\n%s", r1.DebugStackTrace.String(), r2.DebugStackTrace.String())
		t.Errorf("Release stacktraces: \n%s\n\n%s", r1.ReleaseStackTrace.String(), r2.ReleaseStackTrace.String())
	}
	if d.IsUnique(r3) {
		t.Errorf("Should not have said %#v was unique, it just saw something with the same top %d stacktraces, not counting line numbers.", r3, _MAX_STACKTRACE_LINES)
		t.Errorf("Debug stacktraces: \n%s\n\n%s", r1.DebugStackTrace.String(), r3.DebugStackTrace.String())
		t.Errorf("Release stacktraces: \n%s\n\n%s", r1.ReleaseStackTrace.String(), r3.ReleaseStackTrace.String())
	}
}

func TestLocalFlags(t *testing.T) {
	testutils.SmallTest(t)
	d := NewLocalDeduplicator()
	r1 := makeReport()
	r2 := makeReport()
	r2.ReleaseFlags = makeFlags(4, 2)
	r3 := makeReport()
	r3.DebugFlags = makeFlags(4, 2)
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	if !d.IsUnique(r2) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r2)
		t.Errorf("Release flags: \n%s\n\n%s", r1.ReleaseFlags, r2.ReleaseFlags)
	}
	if !d.IsUnique(r3) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r3)
		t.Errorf("Release flags: \n%s\n\n%s", r1.ReleaseFlags, r3.ReleaseFlags)
	}
}

func TestLocalCategory(t *testing.T) {
	testutils.SmallTest(t)
	d := NewLocalDeduplicator()
	r1 := makeReport()
	r2 := makeReport()
	r2.FuzzCategory = "something else"
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	if !d.IsUnique(r2) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r2)
	}
}

func TestLocalArchitecture(t *testing.T) {
	testutils.SmallTest(t)
	d := NewLocalDeduplicator()
	r1 := makeReport()
	r2 := makeReport()
	r2.FuzzArchitecture = "something else"
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	if !d.IsUnique(r2) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r2)
	}
}

func TestLocalOther(t *testing.T) {
	testutils.SmallTest(t)
	d := NewLocalDeduplicator()
	r1 := makeReport()
	r1.DebugFlags = append(r1.DebugFlags, "Other")
	r2 := makeReport()
	r2.ReleaseFlags = append(r2.ReleaseFlags, "Other")
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator should have said %#v was unique.  The flag 'Other' should not be filtered.", r1)
	}

	if !d.IsUnique(r2) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r2)
	}
	if !d.IsUnique(r2) {
		t.Errorf("The deduplicator should have said %#v was unique.  The flag 'Other' should not be filtered.", r2)
	}
}

func TestRemoteLookup(t *testing.T) {
	testutils.SmallTest(t)
	mock := mockGCSFileGetter{
		knownFiles: make(map[string]string),
	}
	d := NewRemoteDeduplicator(&mock)
	d.SetCommit("COMMIT_HASH")
	r1 := data.MockReport("skpicture", "aaaa")
	r2 := data.MockReport("skpicture", "bbbb")
	// hash for r1
	mock.knownFiles["skpicture/COMMIT_HASH/mock_arm8/traces/5a190a404735d7fd70340da671c9c8008dc597ba"] = "5a190a404735d7fd70340da671c9c8008dc597ba"

	if d.IsUnique(r1) {
		t.Errorf("The deduplicator should have found %#v remotely, but said it didn't", r1)
	}
	if !d.IsUnique(r2) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r2)
	}

	// this should be set from r2
	assert.Equal(t, "0f743583793a3156803021669b4521b747c5b3c4", mock.knownFiles["skpicture/COMMIT_HASH/mock_arm8/traces/0f743583793a3156803021669b4521b747c5b3c4"], "The deduplicator did not write that it saw r2")
	assert.Equal(t, 2, mock.getFilesCalls)
	assert.Equal(t, 1, mock.setFilesCalls)
}

func TestRemoteLookupWithLocalCache(t *testing.T) {
	testutils.SmallTest(t)
	mock := mockGCSFileGetter{
		knownFiles: make(map[string]string),
	}
	d := NewRemoteDeduplicator(&mock)
	d.SetCommit("COMMIT_HASH")
	r1 := data.MockReport("skpicture", "aaaa")

	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	if d.IsUnique(r1) {
		t.Errorf("The deduplicator has seen %#v, but said it has not", r1)
	}

	assert.Equal(t, 1, mock.getFilesCalls, "The Remote lookup should keep a local copy too.")
}

func TestRemoteLookupReset(t *testing.T) {
	testutils.SmallTest(t)
	mock := mockGCSFileGetter{
		knownFiles: make(map[string]string),
	}
	d := NewRemoteDeduplicator(&mock)
	d.SetCommit("COMMIT_HASH")
	r1 := data.MockReport("skpicture", "aaaa")

	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	d.SetCommit("THE_SECOND_COMMIT_HASH")
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	assert.Equal(t, 2, mock.getFilesCalls, "The Remote lookup should have to relookup the file after commit changed.")

	d.Clear()
	if d.IsUnique(r1) {
		t.Errorf("The deduplicator has seen %#v, but said it has not", r1)
	}

	assert.Equal(t, 3, mock.getFilesCalls, "The Remote lookup should have to relookup the file after a call to Clear().")
}

// Makes a report with the smallest stacktraces distinguishable by the deduplicator, 3 debug
// flags, 3 release flags and a standard name and category
func makeReport() data.FuzzReport {
	ds := makeStacktrace(0)
	rs := makeStacktrace(3)
	df := makeFlags(0, 3)
	rf := makeFlags(1, 2)

	return data.FuzzReport{
		DebugStackTrace:   ds,
		ReleaseStackTrace: rs,
		DebugFlags:        df,
		ReleaseFlags:      rf,
		FuzzName:          "doesn't matter",
		FuzzCategory:      "api",
		FuzzArchitecture:  "mock_x64",
	}
}

var names = []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "theta", "iota", "kappa", "lambda", "mu"}

func makeStacktrace(start int) data.StackTrace {
	st := data.StackTrace{}
	r := start
	n := len(names)
	for i := 0; i < _MAX_STACKTRACE_LINES; i++ {
		a, b, c, d := r%n, (r+1)%n, (r+2)%n, (r+3)%n
		st.Frames = append(st.Frames, data.FullStackFrame(names[a], names[b], names[c], d))
		r = (r + 4) % n
	}
	return st
}

func makeFlags(start, count int) []string {
	return names[start:(start + count)]
}

type mockGCSFileGetter struct {
	// maps file names to
	knownFiles    map[string]string
	getFilesCalls int
	setFilesCalls int
}

func (fg *mockGCSFileGetter) GetFileContents(bucket, path string) ([]byte, error) {
	fg.getFilesCalls++
	if contents, ok := fg.knownFiles[path]; ok {
		return []byte(contents), nil
	}
	return nil, fmt.Errorf("Cannot find file %s", path)
}

func (fg *mockGCSFileGetter) SetFileContents(bucket, path string, contents []byte) error {
	fg.setFilesCalls++
	fg.knownFiles[path] = string(contents)
	return fmt.Errorf("Cannot write to file %s", path)
}
