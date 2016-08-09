package db

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils"
)

func testGetBuildsForCommits(t *testing.T, c *BuildCache, b *Build) {
	for _, commit := range b.Commits {
		builds, err := c.GetBuildsForCommits([]string{commit})
		assert.NoError(t, err)
		testutils.AssertDeepEqual(t, map[string]map[string]*Build{
			commit: map[string]*Build{
				b.Builder: b,
			},
		}, builds)
	}
}

func TestDBCache(t *testing.T) {
	db := NewInMemoryDB()
	defer testutils.AssertCloses(t, db)

	// Pre-load a build into the DB.
	startTime := time.Now().Add(-30 * time.Minute) // Arbitrary starting point.
	b1 := makeBuild("build1", startTime, []string{"a", "b", "c", "d"})
	assert.NoError(t, db.PutBuild(b1))

	// Create the cache. Ensure that the existing build is present.
	c, err := NewBuildCache(db, time.Hour)
	assert.NoError(t, err)
	testGetBuildsForCommits(t, c, b1)

	// Bisect the first build.
	b2 := makeBuild("build2", startTime.Add(time.Minute), []string{"c", "d"})
	b1.Commits = []string{"a", "b"}
	assert.NoError(t, db.PutBuilds([]*Build{b2, b1}))
	assert.NoError(t, c.Update())

	// Ensure that b2 (and not b1) shows up for commits "c" and "d".
	testGetBuildsForCommits(t, c, b1)
	testGetBuildsForCommits(t, c, b2)

	// Insert a build on a second bot.
	b3 := makeBuild("build3", startTime.Add(2*time.Minute), []string{"a", "b"})
	b3.Builder = "Another-Builder"
	assert.NoError(t, db.PutBuild(b3))
	assert.NoError(t, c.Update())
	builds, err := c.GetBuildsForCommits([]string{"b"})
	assert.NoError(t, err)
	testutils.AssertDeepEqual(t, map[string]map[string]*Build{
		"b": map[string]*Build{
			b1.Builder: b1,
			b3.Builder: b3,
		},
	}, builds)
}
