package data

import (
	"path/filepath"
	"reflect"
	"testing"

	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestParseReleaseDump(t *testing.T) {
	unittest.SmallTest(t)
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
			FullStackFrame("", common.ASSEMBLY_CODE_FILE, "_start", -1),
		},
	}
	if !reflect.DeepEqual(expected, trace) {
		t.Errorf("Expected %#v\nbut was %#v", expected, trace)
		t.Errorf("Expected \n%s\nbut was \n%s", expected.String(), trace.String())
	}
}

func TestParseDebugDump(t *testing.T) {
	unittest.SmallTest(t)
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
			FullStackFrame("", common.ASSEMBLY_CODE_FILE, "_start", -1),
		},
	}

	if !reflect.DeepEqual(expected, trace) {
		t.Errorf("Expected %#v\nbut was %#v", expected, trace)
		t.Errorf("Expected \n%s\nbut was \n%s", expected.String(), trace.String())
	}
}

func TestParsingEdgeCases(t *testing.T) {
	unittest.SmallTest(t)
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
			FullStackFrame("", common.ASSEMBLY_CODE_FILE, "_start", -1),
		},
	}

	if !reflect.DeepEqual(expected, trace) {
		t.Errorf("Expected %#v\nbut was %#v", expected, trace)
		t.Errorf("Expected \n%s\nbut was \n%s", expected.String(), trace.String())
	}
}

func TestParseASANSingle(t *testing.T) {
	unittest.SmallTest(t)
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
			FullStackFrame("", common.ASSEMBLY_CODE_FILE, "_start", common.UNKNOWN_LINE),
		},
	}

	if !reflect.DeepEqual(expected, trace) {
		t.Errorf("Expected %#v\nbut was %#v", expected, trace)
		t.Errorf("Expected %s \n but was %s", expected.String(), trace.String())
	}
}

func TestParseASANDouble(t *testing.T) {
	unittest.SmallTest(t)
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
			FullStackFrame("", common.ASSEMBLY_CODE_FILE, "_start", common.UNKNOWN_LINE),
		},
	}

	if !reflect.DeepEqual(expected, trace) {
		t.Errorf("Expected %#v\nbut was %#v", expected, trace)
		t.Errorf("Expected %s \n but was %s", expected.String(), trace.String())
	}
}

func TestParseEmptyStackTrace(t *testing.T) {
	unittest.SmallTest(t)
	trace := parseCatchsegvStackTrace("")

	if !trace.IsEmpty() {
		t.Errorf("Expected stacktrace to be empty but was %#v", trace)
	}
}

func stacktrace(file string) string {
	return filepath.Join("stacktrace", file)
}

var NO_STACKTRACES = map[string]string{"CLANG_DEBUG": "", "CLANG_RELEASE": "", "ASAN_DEBUG": "", "ASAN_RELEASE": ""}

func assertExpectations(t *testing.T, result FuzzResult, ef map[string]FuzzFlag, est map[string]string) {
	for c, expectedFlags := range ef {
		if flags := result.Configs[c].Flags; flags != expectedFlags {
			t.Errorf("Parsed %s flags were wrong.  Expected %s, but was %s", c, expectedFlags.String(), flags.String())
		}
	}
	for c, expectedTopTrace := range est {
		if st := result.Configs[c].StackTrace; expectedTopTrace == "" {
			if !st.IsEmpty() {
				t.Errorf("Config %s should not have had empty stacktrace for %s: st", c, st.String())
			}
		} else if st.IsEmpty() {
			t.Errorf("Empty stacktrace found in config %s, where one should not have been: %#v", c, st)
		} else if st.Frames[0].String() != expectedTopTrace {
			t.Errorf("parsed stacktrace for config %s is wrong %s \n %s", c, expectedTopTrace, st.String())
		}
	}
}

func TestParseGCSPackage_Grey(t *testing.T) {
	unittest.SmallTest(t)
	// Everything was successful or partially successful
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("0grey_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("0grey_debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("0grey_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("0grey_release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   TerminatedGracefully,
		"CLANG_RELEASE": TerminatedGracefully,
		"ASAN_DEBUG":    TerminatedGracefully,
		"ASAN_RELEASE":  TerminatedGracefully,
	}
	assertExpectations(t, result, expectations, nil)
}

func TestParseGCSPackage_GlobalStackOverflow(t *testing.T) {
	unittest.SmallTest(t)
	// Both debug/release crashed with a stackoverflow.
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("1bad_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("1grey_debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("1bad_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("1grey_release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   TerminatedGracefully,
		"CLANG_RELEASE": TerminatedGracefully,
		"ASAN_DEBUG":    ASANCrashed | ASAN_GlobalBufferOverflow,
		"ASAN_RELEASE":  ASANCrashed | ASAN_GlobalBufferOverflow,
	}

	topTrace := map[string]string{
		"CLANG_DEBUG":   "",
		"CLANG_RELEASE": "",
		"ASAN_DEBUG":    "src/codec/SkMasks.cpp:54 convert_to_8",
		"ASAN_RELEASE":  "src/codec/SkMasks.cpp:55 convert_to_8",
	}
	assertExpectations(t, result, expectations, topTrace)
}

func TestParseGCSPackage_AssertDuringRendering(t *testing.T) {
	unittest.SmallTest(t)
	// Debug assert hit.  Release heap buffer overflow in only ASAN
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("2bad_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": testutils.MustReadFile(stacktrace("2bad_debug.dump")),
					"stderr": testutils.MustReadFile(stacktrace("2bad_debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("2bad_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("2grey_release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   AssertionViolated | ClangCrashed,
		"CLANG_RELEASE": TerminatedGracefully,
		"ASAN_DEBUG":    ASANCrashed | AssertionViolated,
		"ASAN_RELEASE":  ASANCrashed | ASAN_HeapBufferOverflow,
	}
	topTrace := map[string]string{
		"CLANG_DEBUG":   "src/core/SkReader32.h:87 skip",
		"CLANG_RELEASE": "",
		"ASAN_DEBUG":    "src/core/SkReader32.h:87 SkReader32::skip",
		"ASAN_RELEASE":  "src/core/SkReader32.h:57 SkReader32::readInt",
	}

	assertExpectations(t, result, expectations, topTrace)
}

func TestParseGCSPackage_UseAfterFree(t *testing.T) {
	unittest.SmallTest(t)
	// Debug ClangCrashed.  Release heap use after free.
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("3grey_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": testutils.MustReadFile(stacktrace("3bad_debug.dump")),
					"stderr": "",
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("3bad_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": testutils.MustReadFile(stacktrace("3bad_release.dump")),
					"stderr": "",
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   ClangCrashed,
		"CLANG_RELEASE": ClangCrashed,
		"ASAN_DEBUG":    TerminatedGracefully,
		"ASAN_RELEASE":  ASANCrashed | ASAN_HeapUseAfterFree,
	}
	topTrace := map[string]string{
		"CLANG_DEBUG":   "src/codec/SkMasks.cpp:55 convert_to_8_dump",
		"CLANG_RELEASE": "src/codec/SkMasks.cpp:54 convert_to_8_dump",
		"ASAN_DEBUG":    "",
		"ASAN_RELEASE":  "src/codec/SkMasks.cpp:54 convert_to_8_asan",
	}

	assertExpectations(t, result, expectations, topTrace)
}

func TestParseGCSPackage_TimeOut(t *testing.T) {
	unittest.SmallTest(t)
	// Everything timed out on analysis
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("4bad_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("4bad_debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("4bad_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("4bad_release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   TimedOut,
		"CLANG_RELEASE": TimedOut,
		"ASAN_DEBUG":    TimedOut,
		"ASAN_RELEASE":  TimedOut,
	}

	assertExpectations(t, result, expectations, NO_STACKTRACES)
}

func TestParseGCSPackage_BadAlloc(t *testing.T) {
	unittest.SmallTest(t)
	// Everything was a bad:alloc
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("5bad_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("5bad_debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("5bad_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("5bad_release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   BadAlloc,
		"CLANG_RELEASE": BadAlloc,
		"ASAN_DEBUG":    BadAlloc,
		"ASAN_RELEASE":  BadAlloc,
	}
	topTrace := map[string]string{
		"CLANG_DEBUG":   "",
		"CLANG_RELEASE": "",
		"ASAN_DEBUG":    "src/core/SkPictureData.cpp:377 SkPictureData::parseStreamTag",
		"ASAN_RELEASE":  "src/core/SkPictureData.cpp:378 SkPictureData::parseStreamTag",
	}

	assertExpectations(t, result, expectations, topTrace)
}

func TestParseGCSPackage_EmptyStacktrace(t *testing.T) {
	unittest.SmallTest(t)
	// According to AddressSanitizer, both crashed while trying to report a bug.
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("6bad_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": testutils.MustReadFile(stacktrace("6bad_debug.dump")),
					"stderr": testutils.MustReadFile(stacktrace("6bad_debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("6bad_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": testutils.MustReadFile(stacktrace("6bad_release.dump")),
					"stderr": testutils.MustReadFile(stacktrace("6bad_release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   NoStackTrace | ClangCrashed,
		"CLANG_RELEASE": NoStackTrace | ClangCrashed,
		"ASAN_DEBUG":    NoStackTrace | ASANCrashed,
		"ASAN_RELEASE":  NoStackTrace | ASANCrashed,
	}

	assertExpectations(t, result, expectations, NO_STACKTRACES)
}

func TestParseGCSPackage_SKAbort(t *testing.T) {
	unittest.SmallTest(t)
	// According to AddressSanitizer, both crashed while trying to report a bug.
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("7bad_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("7bad_debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("7bad_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("7bad_release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   SKAbortHit,
		"CLANG_RELEASE": SKAbortHit,
		"ASAN_DEBUG":    SKAbortHit,
		"ASAN_RELEASE":  SKAbortHit,
	}
	topTrace := map[string]string{
		"CLANG_DEBUG":   "src/core/SkPictureData.cpp:362 UNKNOWN",
		"CLANG_RELEASE": "src/core/SkPictureData.cpp:364 UNKNOWN",
		"ASAN_DEBUG":    "src/core/SkPictureData.cpp:361 UNKNOWN",
		"ASAN_RELEASE":  "src/core/SkPictureData.cpp:363 UNKNOWN",
	}

	assertExpectations(t, result, expectations, topTrace)
}

func TestParseGCSPackage_SKBoring(t *testing.T) {
	unittest.SmallTest(t)
	// Everything triggered SkBoring.
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("8grey_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("8grey_debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("8grey_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("8grey_release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   TerminatedGracefully,
		"CLANG_RELEASE": TerminatedGracefully,
		"ASAN_DEBUG":    TerminatedGracefully,
		"ASAN_RELEASE":  TerminatedGracefully,
	}
	assertExpectations(t, result, expectations, NO_STACKTRACES)
}

func TestParseGCSPackage_ClangDumpedNoSymbols(t *testing.T) {
	unittest.SmallTest(t)
	// Release dumped for Clang only, and there were no symbols. Also, only Clang hit the assert.
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("9bad_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("9bad_debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("9bad_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": testutils.MustReadFile(stacktrace("9bad_release.dump")),
					"stderr": testutils.MustReadFile(stacktrace("9bad_release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   TerminatedGracefully,
		"CLANG_RELEASE": ClangCrashed,
		"ASAN_DEBUG":    SKAbortHit,
		"ASAN_RELEASE":  TerminatedGracefully,
	}
	topTrace := map[string]string{
		"CLANG_DEBUG":   "",
		"CLANG_RELEASE": "UNSYMBOLIZEDUNSYMBOLIZED:-1 LinearGradientContext::shade4_dx_clamp",
		"ASAN_DEBUG":    "src/effects/gradients/SkLinearGradient.cpp:496 UNKNOWN",
		"ASAN_RELEASE":  "",
	}
	assertExpectations(t, result, expectations, topTrace)
}

func TestParseGCSPackage_BadAllocNoCrash(t *testing.T) {
	unittest.SmallTest(t)
	// Bad alloc, but no stack trace, only the last frame.
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("10debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("10debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("10release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("10release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   BadAlloc,
		"CLANG_RELEASE": BadAlloc,
		"ASAN_DEBUG":    BadAlloc,
		"ASAN_RELEASE":  BadAlloc,
	}
	topTrace := map[string]string{
		"CLANG_DEBUG":   "",
		"CLANG_RELEASE": "",
		"ASAN_DEBUG":    "src/core/SkPictureData.cpp:392 UNKNOWN",
		"ASAN_RELEASE":  "src/core/SkPictureData.cpp:377 SkPictureData::parseStreamTag",
	}
	assertExpectations(t, result, expectations, topTrace)
}

func TestParseGCSPackage_AssemblyCrash(t *testing.T) {
	unittest.SmallTest(t)
	// The crash was in a line of assembly code, which lacks file information
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("11bad_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": testutils.MustReadFile(stacktrace("11bad_debug.dump")),
					"stderr": testutils.MustReadFile(stacktrace("11bad_debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("11bad_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": testutils.MustReadFile(stacktrace("11bad_release.dump")),
					"stderr": testutils.MustReadFile(stacktrace("11bad_release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   ClangCrashed,
		"CLANG_RELEASE": ClangCrashed,
		"ASAN_DEBUG":    ASANCrashed,
		"ASAN_RELEASE":  ASANCrashed,
	}
	topTrace := map[string]string{
		"CLANG_DEBUG":   "[assembly code]:-1 _sk_evenly_spaced_gradient_sse2",
		"CLANG_RELEASE": "[assembly code]:-1 _sk_evenly_spaced_gradient_sse2",
		"ASAN_DEBUG":    "[assembly code]:-1 _sk_evenly_spaced_gradient_sse2",
		"ASAN_RELEASE":  "[assembly code]:-1 _sk_evenly_spaced_gradient_sse2",
	}
	assertExpectations(t, result, expectations, topTrace)
}

func TestParseGCSPackage_StdoutButNoCrash(t *testing.T) {
	unittest.SmallTest(t)
	// There is stuff in dump, but it isn't a crash, so don't say it is
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("12grey_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": testutils.MustReadFile(stacktrace("12grey_debug.dump")),
					"stderr": testutils.MustReadFile(stacktrace("12grey_debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("12grey_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": testutils.MustReadFile(stacktrace("12grey_release.dump")),
					"stderr": testutils.MustReadFile(stacktrace("12grey_release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   TerminatedGracefully,
		"CLANG_RELEASE": TerminatedGracefully,
		"ASAN_DEBUG":    TerminatedGracefully,
		"ASAN_RELEASE":  TerminatedGracefully,
	}
	assertExpectations(t, result, expectations, NO_STACKTRACES)
	if result.IsGrey() != true {
		t.Errorf("Should have been categorized as grey!")
	}
}

func TestParseGCSPackage_ASAN_OOM(t *testing.T) {
	unittest.SmallTest(t)
	// ASAN ran out of memory - grey
	g := GCSPackage{
		Files: map[string]OutputFiles{
			"ASAN_DEBUG": {
				Key: "ASAN_DEBUG",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("13grey_debug.asan")),
				},
			},
			"CLANG_DEBUG": {
				Key: "CLANG_DEBUG",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("13grey_debug.err")),
				},
			},
			"ASAN_RELEASE": {
				Key: "ASAN_RELEASE",
				Content: map[string]string{
					"stderr": testutils.MustReadFile(stacktrace("13grey_release.asan")),
				},
			},
			"CLANG_RELEASE": {
				Key: "CLANG_RELEASE",
				Content: map[string]string{
					"stdout": "",
					"stderr": testutils.MustReadFile(stacktrace("13grey_release.err")),
				},
			},
		},
		FuzzCategory:     "skcodec",
		FuzzArchitecture: "mock_arm8",
	}

	result := ParseGCSPackage(g)
	expectations := map[string]FuzzFlag{
		"CLANG_DEBUG":   TerminatedGracefully,
		"CLANG_RELEASE": TerminatedGracefully,
		"ASAN_DEBUG":    BadAlloc,
		"ASAN_RELEASE":  BadAlloc,
	}
	topTrace := map[string]string{
		"CLANG_DEBUG":   "",
		"CLANG_RELEASE": "",
		"ASAN_DEBUG":    "include/c++/v1/memory:1548 std::__1::allocator_traits",
		"ASAN_RELEASE":  "include/c++/v1/memory:1548 std::__1::allocator_traits",
	}
	assertExpectations(t, result, expectations, topTrace)
	if result.IsGrey() != true {
		t.Errorf("Should have been categorized as grey!")
	}
}
