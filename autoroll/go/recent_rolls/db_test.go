package recent_rolls

import (
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/testutils"
)

// testDB is a struct used for testing database operations.
type testDB struct {
	db  *db
	dir string
}

// newTestDB returns a testDB instance. The caller should call cleanup() on it
// when finished.
func newTestDB(t *testing.T) *testDB {
	tmpDir, err := ioutil.TempDir("", "test_autoroll_db_")
	assert.Nil(t, err)
	dbFile := path.Join(tmpDir, "test.db")
	d, err := openDB(dbFile)
	assert.Nil(t, err)
	return &testDB{
		db:  d,
		dir: tmpDir,
	}
}

// cleanup closes the database and removes the underlying temporary directory.
func (d *testDB) cleanup(t *testing.T) {
	assert.Nil(t, d.db.Close())
	assert.Nil(t, os.RemoveAll(d.dir))
}

// Test that we insert, update, delete, and retrieve rolls as expected.
func TestRolls(t *testing.T) {
	testutils.SkipIfShort(t)
	d := newTestDB(t)
	defer d.cleanup(t)

	now := time.Now().UTC()
	roll1 := &autoroll.AutoRollIssue{
		Closed:      false,
		Committed:   false,
		CommitQueue: true,
		Created:     now,
		Issue:       101101101,
		Modified:    now,
		Patchsets:   []int64{1},
		Result:      autoroll.ROLL_RESULT_IN_PROGRESS,
		Subject:     "Roll asdfdasf",
		TryResults:  []*autoroll.TryResult{},
	}

	// Insert.
	assert.Nil(t, d.db.InsertRoll(roll1))
	test1, err := d.db.GetRoll(roll1.Issue)
	assert.Nil(t, err)
	testutils.AssertDeepEqual(t, roll1, test1)
	recent, err := d.db.GetRecentRolls(10)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(recent))
	testutils.AssertDeepEqual(t, roll1, recent[0])

	// Update.
	roll1.Closed = true
	roll1.Committed = true
	roll1.Result = autoroll.ROLL_RESULT_SUCCESS

	assert.Nil(t, d.db.UpdateRoll(roll1))
	test1, err = d.db.GetRoll(roll1.Issue)
	assert.Nil(t, err)
	testutils.AssertDeepEqual(t, roll1, test1)
	recent, err = d.db.GetRecentRolls(10)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(recent))
	testutils.AssertDeepEqual(t, roll1, recent[0])

	// Delete.
	assert.Nil(t, d.db.DeleteRoll(roll1))
	test1, err = d.db.GetRoll(roll1.Issue)
	assert.Nil(t, err)
	assert.Nil(t, test1)
	recent, err = d.db.GetRecentRolls(10)
	assert.Nil(t, err)
	assert.Equal(t, 0, len(recent))

	// Multiple rolls.
	now = time.Now().UTC().Add(time.Minute)
	roll2 := &autoroll.AutoRollIssue{
		Closed:      false,
		Committed:   false,
		CommitQueue: true,
		Created:     now,
		Issue:       101101102,
		Modified:    now,
		Patchsets:   []int64{1},
		Result:      autoroll.ROLL_RESULT_IN_PROGRESS,
		Subject:     "Roll #2",
		TryResults:  []*autoroll.TryResult{},
	}
	now = time.Now().UTC().Add(30 * time.Minute)
	roll3 := &autoroll.AutoRollIssue{
		Closed:      false,
		Committed:   false,
		CommitQueue: true,
		Created:     now,
		Issue:       101101103,
		Modified:    now,
		Patchsets:   []int64{1},
		Result:      autoroll.ROLL_RESULT_IN_PROGRESS,
		Subject:     "Roll #3",
		TryResults:  []*autoroll.TryResult{},
	}
	now = time.Now().UTC().Add(10 * time.Minute)
	roll4 := &autoroll.AutoRollIssue{
		Closed:      false,
		Committed:   false,
		CommitQueue: true,
		Created:     now,
		Issue:       1001101, // Lower issue number, verify that we order correctly by date.
		Modified:    now,
		Patchsets:   []int64{1},
		Result:      autoroll.ROLL_RESULT_IN_PROGRESS,
		Subject:     "Roll #4",
		TryResults:  []*autoroll.TryResult{},
	}
	for _, roll := range []*autoroll.AutoRollIssue{roll1, roll2, roll3, roll4} {
		assert.Nil(t, d.db.InsertRoll(roll))
	}

	// Ensure that we get the rolls back most recent first.
	expect := []*autoroll.AutoRollIssue{roll3, roll4, roll2, roll1}
	recent, err = d.db.GetRecentRolls(10)
	assert.Nil(t, err)
	testutils.AssertDeepEqual(t, recent, expect)

	// Ensure that we get a maximum of the number of rolls we requested.
	recent, err = d.db.GetRecentRolls(3)
	assert.Nil(t, err)
	testutils.AssertDeepEqual(t, recent, expect[:3])
}
