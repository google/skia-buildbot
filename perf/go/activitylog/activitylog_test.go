package activitylog

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/ds/testutil"
	"go.skia.org/infra/go/testutils"
)

func TestActivity(t *testing.T) {
	testutils.ManualTest(t)
	cleanup := testutil.InitDatastore(t, ds.ACTIVITY)

	defer cleanup()

	// Add one activity.
	a := &Activity{
		UserID: "user@example.com",
		Action: "Triage",
	}
	err := Write(a)
	assert.NoError(t, err)

	// Confirm it's there.
	list, err := GetRecent(2)
	assert.NoError(t, err)
	assert.Len(t, list, 1)

	time.Sleep(3)

	// Add another item.
	a.UserID = "somebody@example.org"
	err = Write(a)
	assert.NoError(t, err)

	// Confirm they're both there.
	list, err = GetRecent(2)
	assert.NoError(t, err)
	assert.Len(t, list, 2)
	assert.Equal(t, "user@example.com", list[0].UserID)
	assert.Equal(t, "somebody@example.org", list[1].UserID)

	// Confirm GetRecent honors its argument.
	list, err = GetRecent(1)
	assert.NoError(t, err)
	assert.Len(t, list, 1)
	assert.Equal(t, "user@example.com", list[0].UserID)
}
