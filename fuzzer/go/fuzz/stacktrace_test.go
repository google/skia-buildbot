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
		Frames: []StackTraceFrame{
			FullStackFrame("src/core/", "SkReadBuffer.cpp", "readFlattenable", 344),
			FullStackFrame("src/core/", "SkReadBuffer.h", "readFlattenable", 130),
			FullStackFrame("src/core/", "SkPictureData.cpp", "parseBufferTag", 498),
			FullStackFrame("src/core/", "SkPictureData.cpp", "parseStreamTag", 424),
			FullStackFrame("src/core/", "SkPictureData.cpp", "parseStream", 580),
			FullStackFrame("src/core/", "SkPicture.cpp", "CreateFromStream", 153),
			FullStackFrame("src/core/", "SkPictureData.cpp", "parseStreamTag", 392),
			FullStackFrame("src/core/", "SkPictureData.cpp", "parseStream", 580),
			FullStackFrame("src/core/", "SkPicture.cpp", "CreateFromStream", 153),
			FullStackFrame("fuzzer_cache/src/", "parseskp.cpp", "tool_main", 41),
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
		Frames: []StackTraceFrame{
			FullStackFrame("src/core/", "SkReadBuffer.cpp", "readFlattenable", 343),
			FullStackFrame("src/core/", "SkReadBuffer.h", "readFlattenable", 130),
			FullStackFrame("src/core/", "SkReadBuffer.h", "readPathEffect", 136),
			FullStackFrame("src/core/", "SkPaint.cpp", "unflatten", 1971),
			FullStackFrame("src/core/", "SkReadBuffer.h", "readPaint", 126),
			FullStackFrame("src/core/", "SkPictureData.cpp", "parseBufferTag", 498),
			FullStackFrame("src/core/", "SkPictureData.cpp", "parseStreamTag", 424),
			FullStackFrame("src/core/", "SkPictureData.cpp", "parseStream", 580),
			FullStackFrame("src/core/", "SkPictureData.cpp", "CreateFromStream", 553),
			FullStackFrame("src/core/", "SkPicture.cpp", "CreateFromStream", 153),
			FullStackFrame("src/core/", "SkPictureData.cpp", "parseStreamTag", 392),
			FullStackFrame("src/core/", "SkPictureData.cpp", "parseStream", 580),
			FullStackFrame("src/core/", "SkPictureData.cpp", "CreateFromStream", 553),
			FullStackFrame("src/core/", "SkPicture.cpp", "CreateFromStream", 153),
			FullStackFrame("src/core/", "SkPicture.cpp", "CreateFromStream", 142),
			FullStackFrame("fuzzer_cache/src/", "parseskp.cpp", "tool_main", 41),
		},
	}

	if !reflect.DeepEqual(expected, trace) {
		t.Errorf("Expected %#v\nbut was %#v", expected, trace)
	}
}

func TestParsingEdgeCases(t *testing.T) {
	// This is a made up dump that has the edge cases for parsing.
	testInput := testutils.MustReadFile("parse-edge.dump")
	trace := ParseStackTrace(testInput)
	expected := StackTrace{
		Frames: []StackTraceFrame{
			FullStackFrame("src/codec/", "SkMasks.cpp", "convert_to_8", 54),
			FullStackFrame("src/codec/", "SkBmpMaskCodec.cpp", "decodeRows", 93),
			FullStackFrame("src/core/", "SkClipStack.cpp", "Element::updateBoundAndGenID", 483),
			FullStackFrame("src/core/", "SkClipStack.cpp", "pushElement", 719),
			FullStackFrame("dm/", "DMSrcSink.cpp", "SKPSrc::draw", 751),
			FullStackFrame("src/core/", "SkReader32.h", "eof", 38),
			FullStackFrame("src/core/", "SkTaskGroup.cpp", "ThreadPool::Wait", 88),
			FullStackFrame("fuzz/", "fuzz.cpp", "fuzz_img", 110),
		},
	}

	if !reflect.DeepEqual(expected, trace) {
		t.Errorf("Expected %#v\nbut was %#v", expected, trace)
		t.Errorf("Expected \n%s\nbut was \n%s", expected.String(), trace.String())
	}
}

func TestParseEmptyStackTrace(t *testing.T) {
	trace := ParseStackTrace("")

	if !trace.IsEmpty() {
		t.Errorf("Expected stacktrace to be empty but was %#v", trace)
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
