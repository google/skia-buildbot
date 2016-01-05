package fuzzcache

import (
	"os"
	"reflect"
	"testing"

	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/fuzz"
	"go.skia.org/infra/go/testutils"
)

const TEST_DB_PATH = "/tmp/test-db"

func TestBoltDBStoreAndRetrieve(t *testing.T) {
	deleteBeforeTest(t)
	db, err := New(TEST_DB_PATH)
	if err != nil {
		t.Fatalf("Could not open test db: %s", err)
	}
	defer testutils.CloseInTest(t, db)
	if err := db.Store(expectedBinaryReport1, expectedBinaryFuzzNames, "deadbeef"); err != nil {
		t.Errorf("Could not store to test db:%s ", err)
	}
	report, names, err := db.Load("deadbeef")
	if err != nil {
		t.Fatalf("Error while loading: %s", err)
	}
	if !reflect.DeepEqual(expectedBinaryReport1, *report) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedBinaryReport1, *report)
	}
	if !reflect.DeepEqual(expectedBinaryFuzzNames, names) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedBinaryFuzzNames, names)
	}
}

func TestBoltDBDoesNotExist(t *testing.T) {
	deleteBeforeTest(t)
	db, err := New(TEST_DB_PATH)
	if err != nil {
		t.Fatalf("Could not open test db: %s", err)
	}
	defer testutils.CloseInTest(t, db)
	if _, _, err := db.Load("deadbeef"); err == nil {
		t.Errorf("Should have seen error, but did not")
	}
}

func deleteBeforeTest(t *testing.T) {
	if err := os.Remove(TEST_DB_PATH); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Could not delete %s: %s", TEST_DB_PATH, err)
	}
}

func makeStacktrace(file, function string, line int) fuzz.StackTrace {
	return fuzz.StackTrace{
		Frames: []fuzz.StackTraceFrame{
			{
				PackageName:  "mock/package/",
				FileName:     file,
				LineNumber:   line,
				FunctionName: function,
			},
		},
	}
}

var expectedBinaryFuzzNames = []string{"aaaa", "bbbb", "cccc", "dddd", "eeee", "ffff", "gggg"}

var mockFlags = []string{"foo", "bar"}

var mockBinaryDetails = map[string]fuzz.BinaryFuzzReport{
	"aaaa": fuzz.BinaryFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "aaaa",
		BinaryType:         "skp",
	},
	"bbbb": fuzz.BinaryFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  fuzz.StackTrace{},
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "bbbb",
		BinaryType:         "skp",
	},
	"cccc": fuzz.BinaryFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "gamma", 26),
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "cccc",
		BinaryType:         "skp",
	},
	"dddd": fuzz.BinaryFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "gamma", 43),
		ReleaseStackTrace:  makeStacktrace("delta", "epsilon", 125),
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "dddd",
		BinaryType:         "png",
	},
	"eeee": fuzz.BinaryFuzzReport{
		DebugStackTrace:    fuzz.StackTrace{},
		ReleaseStackTrace:  fuzz.StackTrace{},
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "eeee",
		BinaryType:         "png",
	},
	"ffff": fuzz.BinaryFuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "ffff",
		BinaryType:         "skp",
	},
	"gggg": fuzz.BinaryFuzzReport{
		DebugStackTrace:    makeStacktrace("delta", "epsilon", 122),
		ReleaseStackTrace:  fuzz.StackTrace{},
		HumanReadableFlags: mockFlags,
		BadBinaryName:      "gggg",
		BinaryType:         "png",
	},
}

var expectedBinaryReport1 = fuzz.FuzzReportTree{
	fuzz.FileFuzzReport{
		FileName: "mock/package/alpha", BinaryCount: 4, ApiCount: 0, Functions: []fuzz.FunctionFuzzReport{
			fuzz.FunctionFuzzReport{
				FunctionName: "beta", BinaryCount: 3, ApiCount: 0, LineNumbers: []fuzz.LineFuzzReport{
					fuzz.LineFuzzReport{
						LineNumber: 16, BinaryCount: 3, ApiCount: 0, BinaryDetails: []fuzz.BinaryFuzzReport{mockBinaryDetails["aaaa"], mockBinaryDetails["bbbb"], mockBinaryDetails["ffff"]}, APIDetails: nil,
					},
				},
			}, fuzz.FunctionFuzzReport{
				FunctionName: "gamma", BinaryCount: 1, ApiCount: 0, LineNumbers: []fuzz.LineFuzzReport{
					fuzz.LineFuzzReport{
						LineNumber: 26, BinaryCount: 1, ApiCount: 0, BinaryDetails: []fuzz.BinaryFuzzReport{mockBinaryDetails["cccc"]}, APIDetails: nil,
					},
				},
			},
		},
	},
	fuzz.FileFuzzReport{
		FileName: "mock/package/delta", BinaryCount: 2, ApiCount: 0, Functions: []fuzz.FunctionFuzzReport{
			fuzz.FunctionFuzzReport{
				FunctionName: "epsilon", BinaryCount: 2, ApiCount: 0, LineNumbers: []fuzz.LineFuzzReport{
					fuzz.LineFuzzReport{
						LineNumber: 122, BinaryCount: 1, ApiCount: 0, BinaryDetails: []fuzz.BinaryFuzzReport{mockBinaryDetails["gggg"]}, APIDetails: nil,
					},
					fuzz.LineFuzzReport{
						LineNumber: 125, BinaryCount: 1, ApiCount: 0, BinaryDetails: []fuzz.BinaryFuzzReport{mockBinaryDetails["dddd"]}, APIDetails: nil,
					},
				},
			},
		},
	},
	fuzz.FileFuzzReport{
		FileName: common.UNKNOWN_FILE, BinaryCount: 1, ApiCount: 0, Functions: []fuzz.FunctionFuzzReport{
			fuzz.FunctionFuzzReport{
				FunctionName: common.UNKNOWN_FUNCTION, BinaryCount: 1, ApiCount: 0, LineNumbers: []fuzz.LineFuzzReport{
					fuzz.LineFuzzReport{
						LineNumber: -1, BinaryCount: 1, ApiCount: 0, BinaryDetails: []fuzz.BinaryFuzzReport{mockBinaryDetails["eeee"]}, APIDetails: nil,
					},
				},
			},
		},
	},
}
