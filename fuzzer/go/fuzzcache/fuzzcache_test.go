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
	if err := db.StoreTree(expectedPictureTree, "skpicture", "deadbeef"); err != nil {
		t.Errorf("Could not store skpicture tree to test db:%s ", err)
	}
	if err := db.StoreTree(expectedAPITree, "api", "deadbeef"); err != nil {
		t.Errorf("Could not store api tree to test db:%s ", err)
	}
	if err := db.StoreFuzzNames(expectedFuzzNames, "deadbeef"); err != nil {
		t.Errorf("Could not store api tree to test db:%s ", err)
	}

	report, err := db.LoadTree("skpicture", "deadbeef")
	if err != nil {
		t.Fatalf("Error while loading skpicture tree: %s", err)
	}
	if !reflect.DeepEqual(expectedPictureTree, *report) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedPictureTree, *report)
	}
	report, err = db.LoadTree("api", "deadbeef")
	if err != nil {
		t.Fatalf("Error while loading api tree: %s", err)
	}
	if !reflect.DeepEqual(expectedAPITree, *report) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedAPITree, *report)
	}
	names, err := db.LoadFuzzNames("deadbeef")

	if !reflect.DeepEqual(expectedFuzzNames, names) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedFuzzNames, names)
	}
}

func TestBoltDBDoesNotExist(t *testing.T) {
	deleteBeforeTest(t)
	db, err := New(TEST_DB_PATH)
	if err != nil {
		t.Fatalf("Could not open test db: %s", err)
	}
	defer testutils.CloseInTest(t, db)
	if _, err := db.LoadFuzzNames("deadbeef"); err == nil {
		t.Errorf("Should have seen error, but did not")
	}
	if _, err := db.LoadTree("api", "deadbeef"); err == nil {
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

var expectedFuzzNames = []string{"aaaa", "bbbb", "cccc", "dddd", "eeee", "ffff", "gggg"}

var mockFlags = []string{"foo", "bar"}

var mockPictureDetails = map[string]fuzz.FuzzReport{
	"aaaa": fuzz.FuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		FuzzName:           "aaaa",
		FuzzCategory:       "skpicture",
	},
	"bbbb": fuzz.FuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  fuzz.StackTrace{},
		HumanReadableFlags: mockFlags,
		FuzzName:           "bbbb",
		FuzzCategory:       "skpicture",
	},
	"cccc": fuzz.FuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "gamma", 26),
		HumanReadableFlags: mockFlags,
		FuzzName:           "cccc",
		FuzzCategory:       "skpicture",
	},
	"dddd": fuzz.FuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "gamma", 43),
		ReleaseStackTrace:  makeStacktrace("delta", "epsilon", 125),
		HumanReadableFlags: mockFlags,
		FuzzName:           "dddd",
		FuzzCategory:       "skpicture",
	},
	"eeee": fuzz.FuzzReport{
		DebugStackTrace:    fuzz.StackTrace{},
		ReleaseStackTrace:  fuzz.StackTrace{},
		HumanReadableFlags: mockFlags,
		FuzzName:           "eeee",
		FuzzCategory:       "skpicture",
	},
	"ffff": fuzz.FuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		FuzzName:           "ffff",
		FuzzCategory:       "skpicture",
	},
	"gggg": fuzz.FuzzReport{
		DebugStackTrace:    makeStacktrace("delta", "epsilon", 122),
		ReleaseStackTrace:  fuzz.StackTrace{},
		HumanReadableFlags: mockFlags,
		FuzzName:           "gggg",
		FuzzCategory:       "skpicture",
	},
}

var mockAPIDetails = map[string]fuzz.FuzzReport{
	"hhhh": fuzz.FuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  makeStacktrace("alpha", "beta", 16),
		HumanReadableFlags: mockFlags,
		FuzzName:           "hhhh",
		FuzzCategory:       "api",
	},
	"iiii": fuzz.FuzzReport{
		DebugStackTrace:    makeStacktrace("alpha", "beta", 16),
		ReleaseStackTrace:  fuzz.StackTrace{},
		HumanReadableFlags: mockFlags,
		FuzzName:           "iiii",
		FuzzCategory:       "api",
	},
}

var expectedPictureTree = fuzz.FuzzReportTree{
	fuzz.FileFuzzReport{
		FileName: "mock/package/alpha", Count: 4, Functions: []fuzz.FunctionFuzzReport{
			fuzz.FunctionFuzzReport{
				FunctionName: "beta", Count: 3, LineNumbers: []fuzz.LineFuzzReport{
					fuzz.LineFuzzReport{
						LineNumber: 16, Count: 3, Details: []fuzz.FuzzReport{mockPictureDetails["aaaa"], mockPictureDetails["bbbb"], mockPictureDetails["ffff"]},
					},
				},
			}, fuzz.FunctionFuzzReport{
				FunctionName: "gamma", Count: 1, LineNumbers: []fuzz.LineFuzzReport{
					fuzz.LineFuzzReport{
						LineNumber: 26, Count: 1, Details: []fuzz.FuzzReport{mockPictureDetails["cccc"]},
					},
				},
			},
		},
	},
	fuzz.FileFuzzReport{
		FileName: "mock/package/delta", Count: 2, Functions: []fuzz.FunctionFuzzReport{
			fuzz.FunctionFuzzReport{
				FunctionName: "epsilon", Count: 2, LineNumbers: []fuzz.LineFuzzReport{
					fuzz.LineFuzzReport{
						LineNumber: 122, Count: 1, Details: []fuzz.FuzzReport{mockPictureDetails["gggg"]},
					},
					fuzz.LineFuzzReport{
						LineNumber: 125, Count: 1, Details: []fuzz.FuzzReport{mockPictureDetails["dddd"]},
					},
				},
			},
		},
	},
	fuzz.FileFuzzReport{
		FileName: common.UNKNOWN_FILE, Count: 1, Functions: []fuzz.FunctionFuzzReport{
			fuzz.FunctionFuzzReport{
				FunctionName: common.UNKNOWN_FUNCTION, Count: 1, LineNumbers: []fuzz.LineFuzzReport{
					fuzz.LineFuzzReport{
						LineNumber: -1, Count: 1, Details: []fuzz.FuzzReport{mockPictureDetails["eeee"]},
					},
				},
			},
		},
	},
}

var expectedAPITree = fuzz.FuzzReportTree{
	fuzz.FileFuzzReport{
		FileName: "mock/package/alpha", Count: 2, Functions: []fuzz.FunctionFuzzReport{
			fuzz.FunctionFuzzReport{
				FunctionName: "beta", Count: 2, LineNumbers: []fuzz.LineFuzzReport{
					fuzz.LineFuzzReport{
						LineNumber: 16, Count: 2, Details: []fuzz.FuzzReport{mockAPIDetails["hhhh"], mockAPIDetails["iiii"]},
					},
				},
			},
		},
	},
}
