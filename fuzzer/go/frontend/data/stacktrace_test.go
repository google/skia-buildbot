package data

import (
	"path/filepath"
	"reflect"
	"testing"

	"go.skia.org/infra/go/testutils"
)

func TestParseReleaseDump(t *testing.T) {
	testInput := testutils.MustReadFile("parse-catchsegv-release.dump")
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
	testInput := testutils.MustReadFile("parse-catchsegv-debug.dump")

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
	// This is a made up dump that has the edge cases for parsing function names.
	testInput := testutils.MustReadFile("parse-catchsegv-edge.dump")
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
