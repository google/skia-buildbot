package db

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/testutils"
)

func makeBuild(id string, ts time.Time, commits []string) *Build {
	return &Build{
		Build: &buildbucket.Build{
			CreatedTimestamp: fmt.Sprintf("%d", ts.UnixNano()),
			Id:               id,
		},
		Builder: "Test-Builder",
		Commits: commits,
	}
}

func testDB(t *testing.T, db DB) {
	defer testutils.AssertCloses(t, db)

	_, err := db.GetModifiedBuilds("dummy-id")
	assert.True(t, IsUnknownId(err))

	id, err := db.StartTrackingModifiedBuilds()
	assert.NoError(t, err)

	builds, err := db.GetModifiedBuilds(id)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(builds))

	// Insert a build.
	b1 := makeBuild("build1", time.Unix(0, 1470674132000000), []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutBuild(b1))

	// Ensure that the build shows up in the modified list.
	builds, err = db.GetModifiedBuilds(id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b1}, builds)

	// Ensure that the build shows up in the correct date ranges.
	timeStart := time.Time{}
	b1Before, err := b1.Created()
	assert.NoError(t, err)
	b1After := b1Before.Add(1 * time.Millisecond)
	timeEnd := time.Now()
	builds, err = db.GetBuildsFromDateRange(timeStart, b1Before)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(builds))
	builds, err = db.GetBuildsFromDateRange(b1Before, b1After)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b1}, builds)
	builds, err = db.GetBuildsFromDateRange(b1After, timeEnd)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(builds))

	// Insert two more builds.
	b2 := makeBuild("build2", time.Unix(0, 1470674376000000), []string{"e", "f"})
	b3 := makeBuild("build3", time.Unix(0, 1470674884000000), []string{"g", "h"})
	assert.NoError(t, db.PutBuilds([]*Build{b2, b3}))

	// Ensure that both builds show up in the modified list.
	builds, err = db.GetModifiedBuilds(id)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b2, b3}, builds)

	// Ensure that all builds show up in the correct time ranges, in sorted order.
	b2Before, err := b2.Created()
	assert.NoError(t, err)
	b2After := b2Before.Add(1 * time.Millisecond)

	b3Before, err := b3.Created()
	assert.NoError(t, err)
	b3After := b3Before.Add(1 * time.Millisecond)

	builds, err = db.GetBuildsFromDateRange(timeStart, b1Before)
	assert.NoError(t, err)
	assert.Equal(t, 0, len(builds))

	builds, err = db.GetBuildsFromDateRange(timeStart, b1After)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b1}, builds)

	builds, err = db.GetBuildsFromDateRange(timeStart, b2Before)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b1}, builds)

	builds, err = db.GetBuildsFromDateRange(timeStart, b2After)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b1, b2}, builds)

	builds, err = db.GetBuildsFromDateRange(timeStart, b3Before)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b1, b2}, builds)

	builds, err = db.GetBuildsFromDateRange(timeStart, b3After)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b1, b2, b3}, builds)

	builds, err = db.GetBuildsFromDateRange(timeStart, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b1, b2, b3}, builds)

	builds, err = db.GetBuildsFromDateRange(b1Before, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b1, b2, b3}, builds)

	builds, err = db.GetBuildsFromDateRange(b1After, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b2, b3}, builds)

	builds, err = db.GetBuildsFromDateRange(b2Before, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b2, b3}, builds)

	builds, err = db.GetBuildsFromDateRange(b2After, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b3}, builds)

	builds, err = db.GetBuildsFromDateRange(b3Before, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{b3}, builds)

	builds, err = db.GetBuildsFromDateRange(b3After, timeEnd)
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, []*Build{}, builds)
}

func testTooManyUsers(t *testing.T, db DB) {
	defer testutils.AssertCloses(t, db)

	// Max out the number of modified-builds users; ensure that we error out.
	for i := 0; i < MAX_MODIFIED_BUILDS_USERS; i++ {
		_, err := db.StartTrackingModifiedBuilds()
		assert.NoError(t, err)
	}
	_, err := db.StartTrackingModifiedBuilds()
	assert.True(t, IsTooManyUsers(err))
}

func TestInMemoryDB(t *testing.T) {
	testDB(t, NewInMemoryDB())
}

func TestInMemoryTooManyUsers(t *testing.T) {
	testTooManyUsers(t, NewInMemoryDB())
}
