package fuzzcache

import (
	"os"
	"reflect"
	"testing"

	"go.skia.org/infra/fuzzer/go/data"
	"go.skia.org/infra/fuzzer/go/frontend/fuzzpool"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

const TEST_DB_PATH = "/tmp/test-db"

func TestBoltDBStoreAndRetrieve(t *testing.T) {
	unittest.MediumTest(t)
	deleteBeforeTest(t)
	db, err := New(TEST_DB_PATH)
	if err != nil {
		t.Fatalf("Could not open test db: %s", err)
	}
	defer testutils.AssertCloses(t, db)
	if err := db.StorePool(expectedFuzzPool, "deadbeef"); err != nil {
		t.Errorf("Could not store pool to test db:%s ", err)
	}
	if err := db.StoreFuzzNames(expectedFuzzNames, "deadbeef"); err != nil {
		t.Errorf("Could not store api tree to test db:%s ", err)
	}

	pool := fuzzpool.New()
	if err = db.LoadPool(pool, "deadbeef"); err != nil {
		t.Fatalf("Error while loading pool: %s", err)
	}
	if !reflect.DeepEqual(expectedFuzzPool, pool) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedFuzzPool, pool)
	}
	names, err := db.LoadFuzzNames("deadbeef")

	if !reflect.DeepEqual(expectedFuzzNames, names) {
		t.Errorf("Expected: %#v\n, but was: %#v", expectedFuzzNames, names)
	}
}

func TestBoltDBDoesNotExist(t *testing.T) {
	unittest.MediumTest(t)
	deleteBeforeTest(t)
	db, err := New(TEST_DB_PATH)
	if err != nil {
		t.Fatalf("Could not open test db: %s", err)
	}
	defer testutils.AssertCloses(t, db)
	if _, err := db.LoadFuzzNames("deadbeef"); err == nil {
		t.Errorf("Should have seen error, but did not")
	}
	pool := fuzzpool.New()
	if err := db.LoadPool(pool, "deadbeef"); err == nil {
		t.Errorf("Should have seen error, but did not")
	}
}

func deleteBeforeTest(t *testing.T) {
	if err := os.Remove(TEST_DB_PATH); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Could not delete %s: %s", TEST_DB_PATH, err)
	}
}

var expectedFuzzNames = []string{"aaaa", "bbbb", "cccc", "dddd", "eeee", "ffff", "gggg"}

var expectedFuzzPool = fuzzpool.NewForTests([]data.FuzzReport{data.MockReport("skpicture", "aaaa"), data.MockReport("skpicture", "bbbb"), data.MockReport("skpicture", "cccc"), data.MockReport("skpicture", "dddd"), data.MockReport("skpicture", "eeee"), data.MockReport("skpicture", "ffff"), data.MockReport("skpicture", "gggg")})
