package deduplicator

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/fuzzer/go/tests"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestLocalSimpleDeduplication(t *testing.T) {
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
	// r1 is a report with 6 and 7 stacktrace frames for Debug/Release
	r1 := makeReport()
	debugStacktrace := data.StackTrace{}
	releaseStacktrace := data.StackTrace{}
	debugStacktrace.Frames = append(r1.Stacktraces["CLANG_DEBUG"].Frames, data.StackTraceFrame{})
	// Intentionally add one frame to the debugStacktrace and set it as release
	releaseStacktrace.Frames = append(debugStacktrace.Frames, data.StackTraceFrame{})

	r1.Stacktraces["CLANG_DEBUG"] = debugStacktrace
	r1.Stacktraces["CLANG_RELEASE"] = releaseStacktrace

	k1 := key(r1)
	k2 := key(r1)
	if k1 != k2 {
		t.Errorf("Keys should be deterministic\n%s != %s", k1, k2)
	}
	if n := len(debugStacktrace.Frames); n != _MAX_STACKTRACE_LINES+1 {
		t.Errorf("key() should not have changed the report - it now has %d frames instead of 6", n)
		t.Errorf(debugStacktrace.String())
	}
	if n := len(releaseStacktrace.Frames); n != _MAX_STACKTRACE_LINES+2 {
		t.Errorf("key() should not have changed the report - it now has %d frames instead of 7", n)
		t.Errorf(debugStacktrace.String())
	}
	if frame := debugStacktrace.Frames[0]; frame.LineNumber == 0 {
		t.Errorf("key() should not have changed the report - it now has the wrong line number at index 0: %s", debugStacktrace.String())
	}
	if frame := releaseStacktrace.Frames[0]; frame.LineNumber == 0 {
		t.Errorf("key() should not have changed the report - it now has the wrong line number at index 0: %s", releaseStacktrace.String())
	}
}

func TestLocalLinesOfStacktrace(t *testing.T) {
	unittest.SmallTest(t)
	d := NewLocalDeduplicator()
	r1 := makeReport()
	r2 := makeReport()
	debugStacktrace := data.StackTrace{}
	debugStacktrace.Frames = append(r2.Stacktraces["CLANG_DEBUG"].Frames, data.StackTraceFrame{})
	r3 := makeReport()
	releaseStacktrace := data.StackTrace{}
	releaseStacktrace.Frames = append(r2.Stacktraces["CLANG_RELEASE"].Frames, data.StackTraceFrame{})
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	if d.IsUnique(r2) {
		t.Errorf("Should not have said %#v was unique, it just saw something with the same top %d stacktraces.", r2, _MAX_STACKTRACE_LINES)
		t.Errorf("r1 stacktraces: \n%#v", r1.Stacktraces)
		t.Errorf("r2 stacktraces: \n%#v", r2.Stacktraces)
	}
	if d.IsUnique(r3) {
		t.Errorf("Should not have said %#v was unique, it just saw something with the same top %d stacktraces.", r3, _MAX_STACKTRACE_LINES)
	}
}

func TestLocalLineNumbers(t *testing.T) {
	unittest.SmallTest(t)
	d := NewLocalDeduplicator()
	r1 := makeReport()
	r2 := makeReport()
	r2.Stacktraces["CLANG_DEBUG"].Frames[0].LineNumber = 9999
	r3 := makeReport()
	r3.Stacktraces["CLANG_RELEASE"].Frames[0].LineNumber = 9999
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	if d.IsUnique(r2) {
		t.Errorf("Should not have said %#v was unique, it just saw something with the same top %d stacktraces, not counting line numbers.", r2, _MAX_STACKTRACE_LINES)
		t.Errorf("r1 stacktraces: \n%#v", r1.Stacktraces)
		t.Errorf("r2 stacktraces: \n%#v", r2.Stacktraces)
	}
	if d.IsUnique(r3) {
		t.Errorf("Should not have said %#v was unique, it just saw something with the same top %d stacktraces, not counting line numbers.", r3, _MAX_STACKTRACE_LINES)
		t.Errorf("r1 stacktraces: \n%#v", r1.Stacktraces)
		t.Errorf("r3 stacktraces: \n%#v", r3.Stacktraces)
	}
}

func TestLocalFlags(t *testing.T) {
	unittest.SmallTest(t)
	d := NewLocalDeduplicator()
	r1 := makeReport()
	r2 := makeReport()
	r2.Flags["CLANG_RELEASE"] = makeFlags(4, 2)
	r3 := makeReport()
	r3.Flags["CLANG_DEBUG"] = makeFlags(4, 2)
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	if !d.IsUnique(r2) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r2)
		t.Errorf("Release flags: \n%s\n\n%s", r1.Flags["CLANG_RELEASE"], r2.Flags["CLANG_RELEASE"])
	}
	if !d.IsUnique(r3) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r3)
		t.Errorf("Release flags: \n%s\n\n%s", r1.Flags["CLANG_RELEASE"], r3.Flags["CLANG_RELEASE"])
	}
}

func TestLocalCategory(t *testing.T) {
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
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
	unittest.SmallTest(t)
	d := NewLocalDeduplicator()
	r1 := makeReport()
	r1.Flags["CLANG_DEBUG"] = append(r1.Flags["CLANG_DEBUG"], "Other")
	r2 := makeReport()
	r2.Flags["CLANG_RELEASE"] = append(r2.Flags["CLANG_RELEASE"], "Other")
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

var ctx = mock.AnythingOfType("*context.emptyCtx")

func TestRemoteLookup(t *testing.T) {
	unittest.SmallTest(t)
	m := tests.NewMockGCSClient()
	defer m.AssertExpectations(t)

	d := NewRemoteDeduplicator(m)
	d.SetRevision("COMMIT_HASH")
	r1 := data.MockReport("skpicture", "aaaa")
	r2 := data.MockReport("skpicture", "bbbb")
	// hash for r1
	// If the key algorithm ever changes, the hash here will need to change as well.
	m.On("GetFileContents", ctx, "skpicture/COMMIT_HASH/mock_arm8/traces/20ed871935578a9deafbc755159e4ac389745a12").Return([]byte("20ed871935578a9deafbc755159e4ac389745a12"), nil)

	if d.IsUnique(r1) {
		t.Errorf("The deduplicator should have found %#v remotely, but said it didn't", r1)
	}
	m.On("GetFileContents", ctx, "skpicture/COMMIT_HASH/mock_arm8/traces/8f5de8e5d95579a935854da4a24b8a42e895e799").Return([]byte(nil), fmt.Errorf("Not found"))

	m.On("SetFileContents", ctx, "skpicture/COMMIT_HASH/mock_arm8/traces/8f5de8e5d95579a935854da4a24b8a42e895e799", FILE_WRITE_OPTS, []byte("8f5de8e5d95579a935854da4a24b8a42e895e799")).Return(nil)
	if !d.IsUnique(r2) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r2)
	}

	m.AssertNumberOfCalls(t, "GetFileContents", 2)
	m.AssertNumberOfCalls(t, "SetFileContents", 1)
}

func TestRemoteLookupWithLocalCache(t *testing.T) {
	unittest.SmallTest(t)
	m := tests.NewMockGCSClient()
	defer m.AssertExpectations(t)

	d := NewRemoteDeduplicator(m)
	d.SetRevision("COMMIT_HASH")
	r1 := data.MockReport("skpicture", "aaaa")

	// If the key algorithm ever changes, the hash will need to change as well.
	m.On("GetFileContents", ctx, "skpicture/COMMIT_HASH/mock_arm8/traces/20ed871935578a9deafbc755159e4ac389745a12").Return([]byte("20ed871935578a9deafbc755159e4ac389745a12"), nil)

	if d.IsUnique(r1) {
		t.Errorf("The deduplicator has seen %#v, but said it has not", r1)
	}
	if d.IsUnique(r1) {
		t.Errorf("The deduplicator has seen %#v, but said it has not", r1)
	}
	if d.IsUnique(r1) {
		t.Errorf("The deduplicator has seen %#v, but said it has not", r1)
	}

	// The Remote lookup should keep a local copy too.
	m.AssertNumberOfCalls(t, "GetFileContents", 1)
}

func TestRemoteLookupReset(t *testing.T) {
	unittest.SmallTest(t)
	m := tests.NewMockGCSClient()
	defer m.AssertExpectations(t)

	d := NewRemoteDeduplicator(m)
	d.SetRevision("COMMIT_HASH")
	r1 := data.MockReport("skpicture", "aaaa")

	// AnythingofType("[]byte") doesn't work because https://github.com/stretchr/testify/issues/387
	m.On("SetFileContents", ctx, mock.AnythingOfType("string"), FILE_WRITE_OPTS, mock.AnythingOfType("[]uint8")).Return(nil)

	// If the key algorithm ever changes, the hash will need to change as well.
	m.On("GetFileContents", ctx, "skpicture/COMMIT_HASH/mock_arm8/traces/20ed871935578a9deafbc755159e4ac389745a12").Return([]byte(nil), fmt.Errorf("Not found"))

	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	m.On("GetFileContents", ctx, "skpicture/THE_SECOND_COMMIT_HASH/mock_arm8/traces/20ed871935578a9deafbc755159e4ac389745a12").Return([]byte(nil), fmt.Errorf("Not found")).Once()

	d.SetRevision("THE_SECOND_COMMIT_HASH")
	if !d.IsUnique(r1) {
		t.Errorf("The deduplicator has not seen %#v, but said it has", r1)
	}
	// The Remote lookup should have to relookup the file after commit changed.
	m.AssertNumberOfCalls(t, "GetFileContents", 2)
	m.AssertNumberOfCalls(t, "SetFileContents", 2)
}

// Makes a report with the smallest stacktraces distinguishable by the deduplicator, 3 debug
// flags, 3 release flags and a standard name and category
func makeReport() data.FuzzReport {
	ds := makeStacktrace(0)
	rs := makeStacktrace(3)
	df := makeFlags(0, 3)
	rf := makeFlags(1, 2)

	return data.FuzzReport{
		Stacktraces: map[string]data.StackTrace{
			"ASAN_DEBUG":    ds,
			"CLANG_DEBUG":   ds,
			"ASAN_RELEASE":  rs,
			"CLANG_RELEASE": rs,
		},
		Flags: map[string][]string{
			"ASAN_DEBUG":    df,
			"CLANG_DEBUG":   df,
			"ASAN_RELEASE":  rf,
			"CLANG_RELEASE": rf,
		},
		FuzzName:         "doesn't matter",
		FuzzCategory:     "api",
		FuzzArchitecture: "mock_x64",
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
