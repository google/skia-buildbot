package fuzz

import (
	"path/filepath"
	"reflect"
	"testing"

	"go.skia.org/infra/go/testutils"
)

func TestParseReleaseDump(t *testing.T) {
	testInput := testutils.MustReadFile("parse-release.dump")

	trace := ParseStackTrace(testInput)

	expected := StackTrace{
		[]StackTraceFrame{
			BasicStackFrame("src/core/", "SkReadBuffer.cpp", 344),
			BasicStackFrame("src/core/", "SkReadBuffer.h", 130),
			BasicStackFrame("src/core/", "SkPictureData.cpp", 498),
			BasicStackFrame("src/core/", "SkPictureData.cpp", 424),
			BasicStackFrame("src/core/", "SkPictureData.cpp", 580),
			BasicStackFrame("src/core/", "SkPicture.cpp", 153),
			BasicStackFrame("src/core/", "SkPictureData.cpp", 392),
			BasicStackFrame("src/core/", "SkPictureData.cpp", 580),
			BasicStackFrame("src/core/", "SkPicture.cpp", 153),
			BasicStackFrame("fuzzer_cache/src/", "parseskp.cpp", 41),
		},
	}

	if !reflect.DeepEqual(expected, trace) {
		t.Errorf("Expected %#v\nbut was %#v", expected, trace)
	}

}

func TestParseDebugDump(t *testing.T) {
	testInput := testutils.MustReadFile("parse-debug.dump")

	trace := ParseStackTrace(testInput)

	expected := StackTrace{
		[]StackTraceFrame{
			BasicStackFrame("src/core/", "SkReadBuffer.h", 130),
			BasicStackFrame("src/core/", "SkReadBuffer.h", 136),
			BasicStackFrame("src/core/", "SkPaint.cpp", 1971),
			BasicStackFrame("src/core/", "SkReadBuffer.h", 126),
			BasicStackFrame("src/core/", "SkPictureData.cpp", 498),
			BasicStackFrame("src/core/", "SkPictureData.cpp", 424),
			BasicStackFrame("src/core/", "SkPictureData.cpp", 580),
			BasicStackFrame("src/core/", "SkPictureData.cpp", 553),
			BasicStackFrame("src/core/", "SkPicture.cpp", 153),
			BasicStackFrame("src/core/", "SkPictureData.cpp", 392),
			BasicStackFrame("src/core/", "SkPictureData.cpp", 580),
			BasicStackFrame("src/core/", "SkPictureData.cpp", 553),
			BasicStackFrame("src/core/", "SkPicture.cpp", 153),
			BasicStackFrame("src/core/", "SkPicture.cpp", 142),
			BasicStackFrame("fuzzer_cache/src/", "parseskp.cpp", 41),
			BasicStackFrame("fuzzer_cache/src/", "parseskp.cpp", 71),
		},
	}

	if !reflect.DeepEqual(expected, trace) {
		t.Errorf("Expected %#v\nbut was %#v", expected, trace)
	}
}

func TestParseEmptyStackTrace(t *testing.T) {
	trace := ParseStackTrace("")

	if !trace.IsEmpty() {
		t.Errorf("Expected stacktrace to be empty but was %#v", trace)
	}
}

func TestLookUpFunctions(t *testing.T) {
	testTrace := StackTrace{
		[]StackTraceFrame{
			BasicStackFrame("src/core/", "SkReadBuffer.h", 130),
			BasicStackFrame("fuzzer_cache/src/", "parseskp.cpp", 165),
		},
	}

	mockFindFunctionName := func(packageName string, fileName string, lineNumber int) string {
		if fileName == "SkReadBuffer.h" {
			return "thisFunction(SkPath path)"
		}
		if fileName == "parseskp.cpp" {
			return "thisOtherFunction()"
		}
		return "PROBLEM"
	}

	testTrace.LookUpFunctions(mockFindFunctionName)

	if testTrace.Frames[0].FunctionName != "thisFunction(SkPath path)" {
		t.Errorf("Wrong function name for frame 0 -> %s", testTrace.Frames[0].FunctionName)
	}
	if testTrace.Frames[1].FunctionName != "thisOtherFunction()" {
		t.Errorf("Wrong function name for frame 1 -> %s", testTrace.Frames[1].FunctionName)
	}
}

func stacktrace(file string) string {
	return filepath.Join("stacktrace", file)
}

func TestParseDumpFilesCase0(t *testing.T) {
	// Case 0, both debug and release dumped, due to an assertion error
	debugDump := testutils.MustReadFile(stacktrace("case0_debug.dump"))
	debugErr := testutils.MustReadFile(stacktrace("case0_debug.err"))
	releaseDump := testutils.MustReadFile(stacktrace("case0_release.dump"))
	releaseErr := testutils.MustReadFile(stacktrace("case0_release.err"))

	result := ParseFuzzResult(debugDump, debugErr, releaseDump, releaseErr)
	expectedFlags := DebugCrashed | ReleaseCrashed | DebugAssertionViolated | ReleaseOther
	if result.Flags != expectedFlags {
		t.Errorf("parsed Flags were wrong.  Expected %s, but was %s", expectedFlags.String(), result.Flags.String())
	}
}

func TestParseDumpFilesCase2(t *testing.T) {
	// Case 2, debug dumped and hit an assertion, release timed out
	debugDump := testutils.MustReadFile(stacktrace("case2_debug.dump"))
	debugErr := testutils.MustReadFile(stacktrace("case2_debug.err"))
	releaseDump := ""
	releaseErr := ""

	result := ParseFuzzResult(debugDump, debugErr, releaseDump, releaseErr)
	expectedFlags := DebugCrashed | DebugAssertionViolated | ReleaseTimedOut
	if result.Flags != expectedFlags {
		t.Errorf("parsed Flags were wrong.  Expected %s, but was %s", expectedFlags.String(), result.Flags.String())
	}
}

func TestParseDumpFilesCase3(t *testing.T) {
	// Case 3, both debug and release ran a bad malloc
	debugDump := testutils.MustReadFile(stacktrace("case3_debug.dump"))
	debugErr := testutils.MustReadFile(stacktrace("case3_debug.err"))
	releaseDump := testutils.MustReadFile(stacktrace("case3_release.dump"))
	releaseErr := testutils.MustReadFile(stacktrace("case3_release.err"))

	result := ParseFuzzResult(debugDump, debugErr, releaseDump, releaseErr)
	expectedFlags := DebugCrashed | DebugBadAlloc | ReleaseCrashed | ReleaseBadAlloc
	if result.Flags != expectedFlags {
		t.Errorf("parsed Flags were wrong.  Expected %s, but was %s", expectedFlags.String(), result.Flags.String())
	}
}

func TestParseDumpFilesCase4(t *testing.T) {
	// Case 4, both debug and release failed gracefully
	debugDump := testutils.MustReadFile(stacktrace("case4_debug.dump"))
	debugErr := testutils.MustReadFile(stacktrace("case4_debug.err"))
	releaseDump := testutils.MustReadFile(stacktrace("case4_release.dump"))
	releaseErr := testutils.MustReadFile(stacktrace("case4_release.err"))

	result := ParseFuzzResult(debugDump, debugErr, releaseDump, releaseErr)
	expectedFlags := DebugFailedGracefully | ReleaseFailedGracefully
	if result.Flags != expectedFlags {
		t.Errorf("parsed Flags were wrong.  Expected %s, but was %s", expectedFlags.String(), result.Flags.String())
	}
}

func TestParseDumpFilesCase5(t *testing.T) {
	// Case 5, both debug and release crashed, but no stacktrace
	debugDump := testutils.MustReadFile(stacktrace("case5_debug.dump"))
	debugErr := testutils.MustReadFile(stacktrace("case5_debug.err"))
	releaseDump := testutils.MustReadFile(stacktrace("case5_release.dump"))
	releaseErr := testutils.MustReadFile(stacktrace("case5_release.err"))

	result := ParseFuzzResult(debugDump, debugErr, releaseDump, releaseErr)
	expectedFlags := DebugCrashed | ReleaseCrashed | DebugNoStackTrace | ReleaseNoStackTrace
	if result.Flags != expectedFlags {
		t.Errorf("parsed Flags were wrong.  Expected %s, but was %s", expectedFlags.String(), result.Flags.String())
	}
}

func TestParseDumpFilesCase6(t *testing.T) {
	// Case 6, both debug and release timed out
	debugDump := ""
	debugErr := ""
	releaseDump := ""
	releaseErr := ""

	result := ParseFuzzResult(debugDump, debugErr, releaseDump, releaseErr)
	expectedFlags := DebugTimedOut | ReleaseTimedOut
	if result.Flags != expectedFlags {
		t.Errorf("parsed Flags were wrong.  Expected %s, but was %s", expectedFlags.String(), result.Flags.String())
	}
}
