package fuzzcache

import (
	"os"
	"reflect"
	"testing"

	"go.skia.org/infra/fuzzer/go/common"
	"go.skia.org/infra/fuzzer/go/frontend/data"
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

var expectedFuzzNames = []string{"aaaa", "bbbb", "cccc", "dddd", "eeee", "ffff", "gggg"}

var expectedPictureTree = data.FuzzReportTree{
	data.FileFuzzReport{
		FileName: "mock/package/alpha", Count: 4, Functions: []data.FunctionFuzzReport{
			data.FunctionFuzzReport{
				FunctionName: "beta", Count: 3, LineNumbers: []data.LineFuzzReport{
					data.LineFuzzReport{
						LineNumber: 16, Count: 3, Details: []data.FuzzReport{data.MockReport("skpicture", "aaaa"), data.MockReport("skpicture", "bbbb"), data.MockReport("skpicture", "ffff")},
					},
				},
			}, data.FunctionFuzzReport{
				FunctionName: "gamma", Count: 1, LineNumbers: []data.LineFuzzReport{
					data.LineFuzzReport{
						LineNumber: 26, Count: 1, Details: []data.FuzzReport{data.MockReport("skpicture", "cccc")},
					},
				},
			},
		},
	},
	data.FileFuzzReport{
		FileName: "mock/package/delta", Count: 2, Functions: []data.FunctionFuzzReport{
			data.FunctionFuzzReport{
				FunctionName: "epsilon", Count: 2, LineNumbers: []data.LineFuzzReport{
					data.LineFuzzReport{
						LineNumber: 122, Count: 1, Details: []data.FuzzReport{data.MockReport("skpicture", "gggg")},
					},
					data.LineFuzzReport{
						LineNumber: 125, Count: 1, Details: []data.FuzzReport{data.MockReport("skpicture", "dddd")},
					},
				},
			},
		},
	},
	data.FileFuzzReport{
		FileName: common.UNKNOWN_FILE, Count: 1, Functions: []data.FunctionFuzzReport{
			data.FunctionFuzzReport{
				FunctionName: common.UNKNOWN_FUNCTION, Count: 1, LineNumbers: []data.LineFuzzReport{
					data.LineFuzzReport{
						LineNumber: -1, Count: 1, Details: []data.FuzzReport{data.MockReport("skpicture", "eeee")},
					},
				},
			},
		},
	},
}

var expectedAPITree = data.FuzzReportTree{
	data.FileFuzzReport{
		FileName: "mock/package/alpha", Count: 2, Functions: []data.FunctionFuzzReport{
			data.FunctionFuzzReport{
				FunctionName: "beta", Count: 2, LineNumbers: []data.LineFuzzReport{
					data.LineFuzzReport{
						LineNumber: 16, Count: 2, Details: []data.FuzzReport{data.MockReport("api", "hhhh"), data.MockReport("api", "iiii")},
					},
				},
			},
		},
	},
}
