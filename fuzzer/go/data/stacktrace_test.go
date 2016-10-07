package data

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/go/testutils"
)

func TestParseReleaseDump(t *testing.T) {
	testInput := testutils.MustReadFile("parse-catchsegv-release.dump")
	trace := parseCatchsegvStackTrace(testInput)
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

	trace := parseCatchsegvStackTrace(testInput)

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
	trace := parseCatchsegvStackTrace(testInput)
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

func TestParseASANSingle(t *testing.T) {
	testInput := testutils.MustReadFile("parse-asan-single.asan")

	trace := parseASANStackTrace(testInput)

	expected := StackTrace{
		Frames: []StackTraceFrame{
			FullStackFrame("src/codec/", "SkMasks.cpp", "convert_to_8", 54),
			FullStackFrame("src/codec/", "SkMaskSwizzler.cpp", "swizzle_mask24_to_n32_opaque", 93),
			FullStackFrame("src/codec/", "SkBmpMaskCodec.cpp", "SkBmpMaskCodec::decodeRows", 103),
			FullStackFrame("third_party/externals/piex/src/", "piex.cc", "piex::GetPreviewData", 59),
			FullStackFrame("third_party/externals/piex/src/", "piex.cc", "piex::GetPreviewData", 68),
			FullStackFrame("fuzz/", "fuzz.cpp", "fuzz_img", 119),
			FullStackFrame("fuzz/", "fuzz.cpp", "main", 53),
		},
	}

	if !reflect.DeepEqual(expected, trace) {
		t.Errorf("Expected %#v\nbut was %#v", expected, trace)
		t.Errorf("Expected %s \n but was %s", expected.String(), trace.String())
	}
}

func TestParseASANDouble(t *testing.T) {
	testInput := testutils.MustReadFile("parse-asan-double.asan")

	trace := parseASANStackTrace(testInput)

	expected := StackTrace{
		Frames: []StackTraceFrame{
			FullStackFrame("src/core/", "SkReader32.h", "SkReader32::readInt_asan", 57),
			FullStackFrame("src/core/", "SkPicturePlayback.cpp", "SkPicturePlayback::handleOp", 151),
			FullStackFrame("src/core/", "SkPicturePlayback.cpp", "SkPicturePlayback::draw", 111),
			FullStackFrame("src/core/", "SkPicture.cpp", "SkPicture::Forwardport", 137),
			FullStackFrame("src/core/", "SkPicture.cpp", "SkPicture::CreateFromStream", 154),
			FullStackFrame("fuzz/", "fuzz.cpp", "fuzz_skp", 143),
			FullStackFrame("fuzz/", "fuzz.cpp", "main", 54),
		},
	}

	if !reflect.DeepEqual(expected, trace) {
		t.Errorf("Expected %#v\nbut was %#v", expected, trace)
		t.Errorf("Expected %s \n but was %s", expected.String(), trace.String())
	}
}

func TestParseEmptyStackTrace(t *testing.T) {
	trace := parseCatchsegvStackTrace("")

	if !trace.IsEmpty() {
		t.Errorf("Expected stacktrace to be empty but was %#v", trace)
	}
}

func stacktrace(file string) string {
	return filepath.Join("stacktrace", file)
}

func TestParseGCSPackage_Grey(t *testing.T) {
	// Everything was successful or partially successful
	g := GCSPackage{
		Debug: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("0grey_debug.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("0grey_debug.err")),
		},
		Release: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("0grey_release.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("0grey_release.err")),
		},
		FuzzCategory: "skcodec",
	}

	result := ParseGCSPackage(g)
	expectedDebugFlags := TerminatedGracefully
	expectedReleaseFlags := TerminatedGracefully
	if result.Debug.Flags != expectedDebugFlags {
		t.Errorf("Parsed Debug flags were wrong.  Expected %s, but was %s", expectedDebugFlags.String(), result.Debug.Flags.String())
	}
	if result.Release.Flags != expectedReleaseFlags {
		t.Errorf("Parsed Release flags were wrong.  Expected %s, but was %s", expectedReleaseFlags.String(), result.Release.Flags.String())
	}
	if !result.Debug.StackTrace.IsEmpty() {
		t.Errorf("Should have had empty debug stacktrace, but was %s", result.Debug.StackTrace.String())
	}
	if !result.Release.StackTrace.IsEmpty() {
		t.Errorf("Should have had empty release stacktrace, but was %s", result.Release.StackTrace.String())
	}
}

// For the following tests, I added the suffix _asan and _dump to the top stacktrace line in the
// raw files, so we can tell where the stacktrace is being parsed from easily, if there are two
// possibilities. The analysis should try to reason about the AddressSanitizer output, with a
// fallback to the catchsegv debug/err output.

func TestParseGCSPackage_GlobalStackOverflow(t *testing.T) {
	// Both debug/release crashed with a stackoverflow.
	g := GCSPackage{
		Debug: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("1bad_debug.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("1grey_debug.err")),
		},
		Release: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("1bad_release.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("1grey_release.err")),
		},
		FuzzCategory: "skcodec",
	}

	result := ParseGCSPackage(g)
	expectedDebugFlags := ASANCrashed | ASAN_GlobalBufferOverflow
	expectedReleaseFlags := ASANCrashed | ASAN_GlobalBufferOverflow
	if result.Debug.Flags != expectedDebugFlags {
		t.Errorf("Parsed Debug flags were wrong.  Expected %s, but was %s", expectedDebugFlags.String(), result.Debug.Flags.String())
	}
	if result.Release.Flags != expectedReleaseFlags {
		t.Errorf("Parsed Release flags were wrong.  Expected %s, but was %s", expectedReleaseFlags.String(), result.Release.Flags.String())
	}
	// There was no catchsegv dump, so only one possibility for stacktrace for both of these.
	if result.Debug.StackTrace.IsEmpty() {
		t.Errorf("Should not have had empty debug stacktrace")
	}
	if result.Release.StackTrace.IsEmpty() {
		t.Errorf("Should not have had empty release stacktrace")
	}
}

func TestParseGCSPackage_AssertDuringRendering(t *testing.T) {
	// Debug assert hit.  Release heap buffer overflow
	g := GCSPackage{
		Debug: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("2bad_debug.asan")),
			Dump:   testutils.MustReadFile(stacktrace("2bad_debug.dump")),
			StdErr: testutils.MustReadFile(stacktrace("2bad_debug.err")),
		},
		Release: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("2bad_release.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("2grey_release.err")),
		},
		FuzzCategory: "skpicture",
	}

	result := ParseGCSPackage(g)
	expectedDebugFlags := ASANCrashed | ClangCrashed | AssertionViolated
	expectedReleaseFlags := ASANCrashed | ASAN_HeapBufferOverflow | SKPICTURE_DuringRendering
	if result.Debug.Flags != expectedDebugFlags {
		t.Errorf("Parsed Debug flags were wrong.  Expected %s, but was %s", expectedDebugFlags.String(), result.Debug.Flags.String())
	}
	if result.Release.Flags != expectedReleaseFlags {
		t.Errorf("Parsed Release flags were wrong.  Expected %s, but was %s", expectedReleaseFlags.String(), result.Release.Flags.String())
	}
	stack := result.Debug.StackTrace
	if stack.IsEmpty() {
		t.Errorf("Should not have had empty debug stacktrace")
	} else {
		if !strings.HasSuffix(stack.Frames[0].FunctionName, "_asan") {
			t.Errorf("Should have parsed stacktrace from asan: \n%s", stack.String())
		}
	}
	// There was no catchsegv dump, so only one possibility for stacktrace.
	if result.Release.StackTrace.IsEmpty() {
		t.Errorf("Should not have had empty release stacktrace")
	}
}

func TestParseGCSPackage_UseAfterFree(t *testing.T) {
	// Debug ClangCrashed.  Release heap use after free.
	g := GCSPackage{
		Debug: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("3grey_debug.asan")),
			Dump:   testutils.MustReadFile(stacktrace("3bad_debug.dump")),
			StdErr: "",
		},
		Release: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("3bad_release.asan")),
			Dump:   testutils.MustReadFile(stacktrace("3bad_release.dump")),
			StdErr: "",
		},
		FuzzCategory: "skpicture",
	}

	result := ParseGCSPackage(g)
	expectedDebugFlags := ClangCrashed
	expectedReleaseFlags := ASANCrashed | ClangCrashed | ASAN_HeapUseAfterFree
	if result.Debug.Flags != expectedDebugFlags {
		t.Errorf("Parsed Debug flags were wrong.  Expected %s, but was %s", expectedDebugFlags.String(), result.Debug.Flags.String())
	}
	if result.Release.Flags != expectedReleaseFlags {
		t.Errorf("Parsed Release flags were wrong.  Expected %s, but was %s", expectedReleaseFlags.String(), result.Release.Flags.String())
	}
	stack := result.Debug.StackTrace
	// There was no asan dump, so only one possibility for stacktrace.
	if stack.IsEmpty() {
		t.Errorf("Should not have had empty debug stacktrace")
	}
	stack = result.Release.StackTrace
	if stack.IsEmpty() {
		t.Errorf("Should not have had empty release stacktrace")
	} else {
		if !strings.HasSuffix(stack.Frames[0].FunctionName, "_asan") {
			t.Errorf("Should have parsed stacktrace from asan: \n%s", stack.String())
		}
	}
}

func TestParseGCSPackage_TimeOut(t *testing.T) {
	// Everything timed out on analysis
	g := GCSPackage{
		Debug: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("4bad_debug.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("4bad_debug.err")),
		},
		Release: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("4bad_release.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("4bad_release.err")),
		},
		FuzzCategory: "skcodec",
	}

	result := ParseGCSPackage(g)
	expectedDebugFlags := TimedOut
	expectedReleaseFlags := TimedOut
	if result.Debug.Flags != expectedDebugFlags {
		t.Errorf("Parsed Debug flags were wrong.  Expected %s, but was %s", expectedDebugFlags.String(), result.Debug.Flags.String())
	}
	if result.Release.Flags != expectedReleaseFlags {
		t.Errorf("Parsed Release flags were wrong.  Expected %s, but was %s", expectedReleaseFlags.String(), result.Release.Flags.String())
	}
	if !result.Debug.StackTrace.IsEmpty() {
		t.Errorf("Should have had empty debug stacktrace, but was %s", result.Debug.StackTrace.String())
	}
	if !result.Release.StackTrace.IsEmpty() {
		t.Errorf("Should have had empty release stacktrace, but was %s", result.Release.StackTrace.String())
	}
}

func TestParseGCSPackage_BadAlloc(t *testing.T) {
	// Everything was a bad:alloc
	g := GCSPackage{
		Debug: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("5bad_debug.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("5bad_debug.err")),
		},
		Release: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("5bad_release.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("5bad_release.err")),
		},
		FuzzCategory: "skcodec",
	}

	result := ParseGCSPackage(g)
	expectedDebugFlags := BadAlloc | ASANCrashed | ClangCrashed
	expectedReleaseFlags := BadAlloc | ASANCrashed | ClangCrashed
	if result.Debug.Flags != expectedDebugFlags {
		t.Errorf("Parsed Debug flags were wrong.  Expected %s, but was %s", expectedDebugFlags.String(), result.Debug.Flags.String())
	}
	if result.Release.Flags != expectedReleaseFlags {
		t.Errorf("Parsed Release flags were wrong.  Expected %s, but was %s", expectedReleaseFlags.String(), result.Release.Flags.String())
	}
	if result.Debug.StackTrace.IsEmpty() {
		t.Errorf("Should not have had empty debug stacktrace")
	}
	if result.Release.StackTrace.IsEmpty() {
		t.Errorf("Should not have had empty release stacktrace")
	}
}

func TestParseGCSPackage_EmptyStacktrace(t *testing.T) {
	// According to AddressSanitizer, both crashed while trying to report a bug.
	g := GCSPackage{
		Debug: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("6bad_debug.asan")),
			Dump:   testutils.MustReadFile(stacktrace("6bad_debug.dump")),
			StdErr: testutils.MustReadFile(stacktrace("6bad_debug.err")),
		},
		Release: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("6bad_release.asan")),
			Dump:   testutils.MustReadFile(stacktrace("6bad_release.dump")),
			StdErr: testutils.MustReadFile(stacktrace("6bad_release.err")),
		},
		FuzzCategory: "skcodec",
	}

	result := ParseGCSPackage(g)
	expectedDebugFlags := NoStackTrace | ASANCrashed | ClangCrashed
	expectedReleaseFlags := NoStackTrace | ASANCrashed | ClangCrashed
	if result.Debug.Flags != expectedDebugFlags {
		t.Errorf("Parsed Debug flags were wrong.  Expected %s, but was %s", expectedDebugFlags.String(), result.Debug.Flags.String())
	}
	if result.Release.Flags != expectedReleaseFlags {
		t.Errorf("Parsed Release flags were wrong.  Expected %s, but was %s", expectedReleaseFlags.String(), result.Release.Flags.String())
	}
	if !result.Debug.StackTrace.IsEmpty() {
		t.Errorf("Should have had empty debug stacktrace")
	}
	if !result.Release.StackTrace.IsEmpty() {
		t.Errorf("Should have had empty release stacktrace")
	}
}

func TestParseGCSPackage_SKAbort(t *testing.T) {
	// Both hit SK_ABORT somewhere.
	g := GCSPackage{
		Debug: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("7bad_debug.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("7bad_debug.err")),
		},
		Release: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("7bad_release.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("7bad_release.err")),
		},
		FuzzCategory: "skpicture",
	}

	result := ParseGCSPackage(g)
	expectedDebugFlags := SKAbortHit | ASANCrashed | ClangCrashed
	expectedReleaseFlags := SKAbortHit | ASANCrashed | ClangCrashed
	if result.Debug.Flags != expectedDebugFlags {
		t.Errorf("Parsed Debug flags were wrong.  Expected %s, but was %s", expectedDebugFlags.String(), result.Debug.Flags.String())
	}
	if result.Release.Flags != expectedReleaseFlags {
		t.Errorf("Parsed Release flags were wrong.  Expected %s, but was %s", expectedReleaseFlags.String(), result.Release.Flags.String())
	}
	if len(result.Debug.StackTrace.Frames) != 1 {
		t.Fatalf("Should have filled in top frame of trace")
	}
	expected := FullStackFrame("src/core/", "SkPictureData.cpp", common.UNKNOWN_FUNCTION, 360)
	if frame := result.Debug.StackTrace.Frames[0]; frame.LineNumber != expected.LineNumber || frame.FunctionName != expected.FunctionName || frame.FileName != expected.FileName || frame.PackageName != expected.PackageName {
		t.Errorf("Top frame was wrong.  Expected: %#v, but was %#v", expected, frame)
	}

	if len(result.Release.StackTrace.Frames) != 1 {
		t.Errorf("Should not have had empty release stacktrace")
	}
	if frame := result.Release.StackTrace.Frames[0]; frame.LineNumber != expected.LineNumber || frame.FunctionName != expected.FunctionName || frame.FileName != expected.FileName || frame.PackageName != expected.PackageName {
		t.Errorf("Top frame was wrong.  Expected: %#v, but was %#v", expected, frame)
	}
}

func TestParseGCSPackage_SKBoring(t *testing.T) {
	// Both triggered SkBoring.
	g := GCSPackage{
		Debug: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("8grey_debug.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("8grey_debug.err")),
		},
		Release: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("8grey_release.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("8grey_release.err")),
		},
		FuzzCategory: "skpicture",
	}

	result := ParseGCSPackage(g)
	expectedDebugFlags := TerminatedGracefully
	expectedReleaseFlags := TerminatedGracefully
	if result.Debug.Flags != expectedDebugFlags {
		t.Errorf("Parsed Debug flags were wrong.  Expected %s, but was %s", expectedDebugFlags.String(), result.Debug.Flags.String())
	}
	if result.Release.Flags != expectedReleaseFlags {
		t.Errorf("Parsed Release flags were wrong.  Expected %s, but was %s", expectedReleaseFlags.String(), result.Release.Flags.String())
	}
	if !result.Debug.StackTrace.IsEmpty() {
		t.Errorf("Should have had empty debug stacktrace")
	}
	if !result.Release.StackTrace.IsEmpty() {
		t.Errorf("Should have had empty release stacktrace")
	}
}

func TestParseGCSPackage_ClangDumpedNoSymbols(t *testing.T) {
	// Release dumped for Clang only, and there were no symbols. Also, only Clang hit the assert.
	g := GCSPackage{
		Debug: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("9bad_debug.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("9bad_debug.err")),
		},
		Release: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("9bad_release.asan")),
			Dump:   testutils.MustReadFile(stacktrace("9bad_release.dump")),
			StdErr: testutils.MustReadFile(stacktrace("9bad_release.err")),
		},
		FuzzCategory: "api_gradient",
	}

	result := ParseGCSPackage(g)
	expectedDebugFlags := ASANCrashed | SKAbortHit
	expectedReleaseFlags := ClangCrashed
	if result.Debug.Flags != expectedDebugFlags {
		t.Errorf("Parsed Debug flags were wrong.  Expected %s, but was %s", expectedDebugFlags.String(), result.Debug.Flags.String())
	}
	if result.Release.Flags != expectedReleaseFlags {
		t.Errorf("Parsed Release flags were wrong.  Expected %s, but was %s", expectedReleaseFlags.String(), result.Release.Flags.String())
	}
	if result.Debug.StackTrace.IsEmpty() {
		t.Errorf("Should not have had empty debug stacktrace")
	}
	if result.Release.StackTrace.IsEmpty() {
		t.Errorf("Should not have had empty release stacktrace")
	}
	frame := result.Release.StackTrace.Frames[0]
	if frame.FunctionName != "LinearGradientContext::shade4_dx_clamp" {
		t.Errorf("Should have parsed unsymbolized stacktrace: \n%s", frame.String())
	}
	if frame.PackageName != common.UNSYMBOLIZED_RESULT {
		t.Errorf("Should have parsed unsymbolized stacktrace: \n%s", frame.String())
	}
}

func TestParseGCSPackage_BadAllocNoCrash(t *testing.T) {
	// Both triggered bad alloc, just with no explicit crash. We don't have a full
	// stacktrace, but we can at least get the most recent line.
	g := GCSPackage{
		Debug: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("10debug.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("10debug.err")),
		},
		Release: OutputFiles{
			Asan:   testutils.MustReadFile(stacktrace("10release.asan")),
			Dump:   "",
			StdErr: testutils.MustReadFile(stacktrace("10release.err")),
		},
		FuzzCategory: "skpicture",
	}

	result := ParseGCSPackage(g)
	expectedDebugFlags := BadAlloc | ClangCrashed | ASANCrashed
	expectedReleaseFlags := BadAlloc | ClangCrashed | ASANCrashed
	if result.Debug.Flags != expectedDebugFlags {
		t.Errorf("Parsed Debug flags were wrong.  Expected %s, but was %s", expectedDebugFlags.String(), result.Debug.Flags.String())
	}
	if result.Release.Flags != expectedReleaseFlags {
		t.Errorf("Parsed Release flags were wrong.  Expected %s, but was %s", expectedReleaseFlags.String(), result.Release.Flags.String())
	}
	expectedDebug := StackTrace{
		Frames: []StackTraceFrame{
			FullStackFrame("src/core/", "SkPictureData.cpp", common.UNKNOWN_FUNCTION, 392),
		},
	}
	if !reflect.DeepEqual(expectedDebug, result.Debug.StackTrace) {
		t.Errorf("Expected Debug to be %#v\nbut was %#v", expectedDebug, result.Debug.StackTrace)
	}
	expectedRelease := StackTrace{
		Frames: []StackTraceFrame{
			FullStackFrame("src/core/", "SkPictureData.cpp", "SkPictureData::parseStreamTag", 377),
		},
	}
	if !reflect.DeepEqual(expectedRelease, result.Release.StackTrace) {
		t.Errorf("Expected Release to be %#v\nbut was %#v", expectedRelease, result.Release.StackTrace)
	}
}
