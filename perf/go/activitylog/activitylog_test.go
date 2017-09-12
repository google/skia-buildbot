package activitylog

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/perf/go/ds"
	"go.skia.org/infra/perf/go/dstestutil"
)

func TestActivity(t *testing.T) {
	testutils.MediumTest(t)
	dstestutil.InitDatastore(t, ds.ACTIVITY)

	Init(true)
	defer Init(false)

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

	// Add another item.
	a.UserID = "somebody@example.org"
	err = Write(a)
	assert.NoError(t, err)

	// Confirm they're both there.
	list, err = GetRecent(2)
	assert.NoError(t, err)
	assert.Len(t, list, 2)

	// Confirm GetRecent honors its argument.
	list, err = GetRecent(1)
	assert.NoError(t, err)
	assert.Len(t, list, 1)
}
