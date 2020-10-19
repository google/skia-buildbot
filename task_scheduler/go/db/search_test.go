package db

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/task_scheduler/go/types"
)

func TestJobSearch(t *testing.T) {
	unittest.SmallTest(t)

	now := time.Now()

	j := types.MakeFullJob(now)
	j.Name = "Build-Win-Clang-x86_64-Debug-Vulkan"

	emptyParams := func() *JobSearchParams {
		start := now.Add(-1 * time.Hour)
		end := now.Add(1 * time.Hour)
		return &JobSearchParams{
			TimeStart: &start,
			TimeEnd:   &end,
		}
	}
	stringPtr := func(s string) *string {
		rv := new(string)
		*rv = s
		return rv
	}
	intPtr := func(i int64) *int64 {
		rv := new(int64)
		*rv = i
		return rv
	}
	boolPtr := func(b bool) *bool {
		rv := new(bool)
		*rv = b
		return rv
	}
	timePtr := func(ts time.Time) *time.Time {
		rv := new(time.Time)
		*rv = ts
		return rv
	}
	matchParams := func() *JobSearchParams {
		return &JobSearchParams{
			Issue:              stringPtr(j.Issue),
			Patchset:           stringPtr(j.Patchset),
			Repo:               stringPtr(j.Repo),
			Revision:           stringPtr(j.Revision),
			BuildbucketBuildID: intPtr(j.BuildbucketBuildId),
			IsForce:            boolPtr(j.IsForce),
			Name:               stringPtr(j.Name),
			Status:             (*types.JobStatus)(stringPtr(string(j.Status))),
			TimeStart:          timePtr(now.Add(-1 * time.Hour)),
			TimeEnd:            timePtr(now.Add(1 * time.Hour)),
		}
	}

	checkMatches := func(p *JobSearchParams) {
		jobs := matchJobs([]*types.Job{j}, p)
		assert.Equal(t, 1, len(jobs))
		assertdeep.Equal(t, j, jobs[0])
	}
	checkNoMatch := func(p *JobSearchParams) {
		jobs := matchJobs([]*types.Job{j}, p)
		assert.Equal(t, 0, len(jobs))
	}

	// Both emptyParams and matchParams should match.
	checkMatches(matchParams())
	checkMatches(emptyParams())
	checkMatches(&JobSearchParams{})

	// Check each individual parameter.

	// Issue
	p := emptyParams()
	p.Issue = stringPtr(j.Issue)
	checkMatches(p)
	p = matchParams()
	p.Issue = stringPtr("bogus")
	checkNoMatch(p)

	// Patchset
	p = emptyParams()
	p.Patchset = stringPtr(j.Patchset)
	checkMatches(p)
	p = matchParams()
	p.Patchset = stringPtr("bogus")
	checkNoMatch(p)

	// Repo
	p = emptyParams()
	p.Repo = stringPtr(j.Repo)
	checkMatches(p)
	p = matchParams()
	p.Repo = stringPtr("bogus")
	checkNoMatch(p)

	// Revision
	p = emptyParams()
	p.Revision = stringPtr(j.Revision)
	checkMatches(p)
	p = matchParams()
	p.Revision = stringPtr("bogus")
	checkNoMatch(p)

	// BuildbucketBuildId
	p = emptyParams()
	p.BuildbucketBuildID = intPtr(j.BuildbucketBuildId)
	checkMatches(p)
	p = matchParams()
	p.BuildbucketBuildID = intPtr(999991)
	checkNoMatch(p)

	// IsForce
	p = emptyParams()
	testIsForce := new(bool)
	*testIsForce = j.IsForce
	p.IsForce = testIsForce
	checkMatches(p)
	p = matchParams()
	*testIsForce = false
	p.IsForce = testIsForce
	checkNoMatch(p)

	// Name
	p = emptyParams()
	p.Name = stringPtr(j.Name)
	checkMatches(p)
	p.Name = stringPtr(j.Name[:3] + ".*")
	checkMatches(p)
	p = matchParams()
	p.Name = stringPtr("bogus")
	checkNoMatch(p)
	p = matchParams()
	p.Name = stringPtr("^T.*")
	checkNoMatch(p)
	p.Name = stringPtr("(((")
	checkNoMatch(p)

	// Status
	p = emptyParams()
	p.Status = (*types.JobStatus)(stringPtr(string(j.Status)))
	checkMatches(p)
	p = matchParams()
	p.Status = (*types.JobStatus)(stringPtr("bogus"))
	checkNoMatch(p)

	// Check time periods.

	// Inclusive TimeStart.
	p = matchParams()
	p.TimeStart = timePtr(j.Created)
	checkMatches(p)

	// j.Created just before p.TimeStart.
	p.TimeStart = timePtr(j.Created.Add(time.Millisecond))
	checkNoMatch(p)

	// Non-inclusive TimeEnd.
	p = matchParams()
	p.TimeEnd = timePtr(j.Created)
	checkNoMatch(p)

	// j.Created Just before TimeEnd.
	p.TimeEnd = timePtr(j.Created.Add(time.Millisecond))
	checkMatches(p)
}

func TestJobSearchParamsJson(t *testing.T) {
	unittest.SmallTest(t)

	decode := func(j string) *JobSearchParams {
		var rv JobSearchParams
		assert.NoError(t, json.Unmarshal([]byte(j), &rv))
		return &rv
	}
	testIsForce := new(bool)
	*testIsForce = true
	p := &JobSearchParams{}
	assertdeep.Equal(t, p, decode(`{}`))

	p.IsForce = testIsForce
	assertdeep.Equal(t, p, decode(`{"is_force": true}`))

	*p.IsForce = false
	assertdeep.Equal(t, p, decode(`{"is_force": false}`))
}
