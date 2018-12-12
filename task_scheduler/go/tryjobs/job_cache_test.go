package tryjobs

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
	memory "go.skia.org/infra/task_scheduler/go/db/memory"
	"go.skia.org/infra/task_scheduler/go/types"
	"go.skia.org/infra/task_scheduler/go/window"
)

func makeJob(created time.Time) *types.Job {
	return &types.Job{
		Created:             created,
		BuildbucketLeaseKey: 12345,
	}
}

func TestJobCache(t *testing.T) {
	testutils.SmallTest(t)
	d := memory.NewInMemoryJobDB(nil)

	// Pre-load a job into the DB.
	now := time.Now()
	j1 := makeJob(now.Add(-10 * time.Minute))
	assert.NoError(t, d.PutJob(j1))

	// Create the cache. Ensure that the existing job is present.
	w, err := window.New(time.Hour, 0, nil)
	assert.NoError(t, err)
	c, err := newTryJobCache(d, w)
	assert.NoError(t, err)
	jobs, err := c.GetActiveTryJobs()
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, []*types.Job{j1}, jobs)

	// Create another job. Ensure that it gets picked up.
	j2 := makeJob(now.Add(-5 * time.Minute))
	assert.NoError(t, d.PutJob(j2))
	assert.NoError(t, c.Update())
	jobs, err = c.GetActiveTryJobs()
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, []*types.Job{j1, j2}, jobs)

	// j1 is not active.
	j1.BuildbucketLeaseKey = 0
	assert.NoError(t, d.PutJob(j1))
	assert.NoError(t, c.Update())
	jobs, err = c.GetActiveTryJobs()
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, []*types.Job{j2}, jobs)

	// Expire j2.
	assert.NoError(t, w.UpdateWithTime(now.Add(time.Hour)))
	assert.NoError(t, c.Update())
	jobs, err = c.GetActiveTryJobs()
	assert.NoError(t, err)
	deepequal.AssertDeepEqual(t, []*types.Job{}, jobs)
}
