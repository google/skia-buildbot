package autoroll

import (
	"fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/buildbucket"
)

func TestTrybotResults(t *testing.T) {
	// Create a fake roll with one in-progress trybot.
	roll := &AutoRollIssue{
		Closed:            false,
		Committed:         false,
		CommitQueue:       true,
		CommitQueueDryRun: true,
		Created:           time.Now(),
		Issue:             123,
		Modified:          time.Now(),
		Patchsets:         []int64{1},
		Subject:           "Roll src/third_party/skia abc123..def456 (3 commits).",
	}
	roll.Result = rollResult(roll)
	from, to, err := rollRev(roll.Subject, func(h string) (string, error) {
		return h, nil
	})
	assert.Nil(t, err)
	roll.RollingFrom = from
	roll.RollingTo = to

	trybot := &buildbucket.Build{
		CreatedTimestamp: fmt.Sprintf("%d", time.Now().UTC().UnixNano()/1000000),
		Status:           TRYBOT_STATUS_STARTED,
		ParametersJson:   "{\"builder_name\":\"fake-builder\"}",
	}
	tryResult, err := TryResultFromBuildbucket(trybot)
	assert.Nil(t, err)
	roll.TryResults = []*TryResult{tryResult}
	assert.False(t, roll.AllTrybotsFinished())
	assert.False(t, roll.AllTrybotsSucceeded())

	// Trybot failed.
	tryResult.Status = TRYBOT_STATUS_COMPLETED
	tryResult.Result = TRYBOT_RESULT_FAILURE
	assert.True(t, roll.AllTrybotsFinished())
	assert.False(t, roll.AllTrybotsSucceeded())

	retry := &buildbucket.Build{
		CreatedTimestamp: fmt.Sprintf("%d", time.Now().UTC().UnixNano()/1000000+25),
		Status:           TRYBOT_STATUS_STARTED,
		ParametersJson:   "{\"builder_name\":\"fake-builder\"}",
	}
	tryResult, err = TryResultFromBuildbucket(retry)
	assert.Nil(t, err)
	roll.TryResults = append(roll.TryResults, tryResult)
	assert.False(t, roll.AllTrybotsFinished())
	assert.False(t, roll.AllTrybotsSucceeded())

	// The second try result, a retry of the first, succeeded.
	tryResult.Status = TRYBOT_STATUS_COMPLETED
	tryResult.Result = TRYBOT_RESULT_SUCCESS
	assert.True(t, roll.AllTrybotsFinished())
	assert.True(t, roll.AllTrybotsSucceeded())

	// Verify that the ordering of try results does not matter.
	roll.TryResults[0], roll.TryResults[1] = roll.TryResults[1], roll.TryResults[0]
	assert.True(t, roll.AllTrybotsFinished())
	assert.True(t, roll.AllTrybotsSucceeded())
}
