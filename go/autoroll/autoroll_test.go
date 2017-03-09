package autoroll

import (
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/jsonutils"
	"go.skia.org/infra/go/testutils"
)

func TestTrybotResults(t *testing.T) {
	testutils.SmallTest(t)
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
	from, to, err := RollRev(roll.Subject, func(h string) (string, error) {
		return h, nil
	})
	assert.NoError(t, err)
	roll.RollingFrom = from
	roll.RollingTo = to

	trybot := &buildbucket.Build{
		Created:        jsonutils.Time(time.Now().UTC()),
		Status:         TRYBOT_STATUS_STARTED,
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"category\":\"cq\"}",
	}
	tryResult, err := TryResultFromBuildbucket(trybot)
	assert.NoError(t, err)
	roll.TryResults = []*TryResult{tryResult}
	assert.False(t, roll.AllTrybotsFinished())
	assert.False(t, roll.AllTrybotsSucceeded())

	// Trybot failed.
	tryResult.Status = TRYBOT_STATUS_COMPLETED
	tryResult.Result = TRYBOT_RESULT_FAILURE
	assert.True(t, roll.AllTrybotsFinished())
	assert.False(t, roll.AllTrybotsSucceeded())

	retry := &buildbucket.Build{
		Created:        jsonutils.Time(time.Now().UTC()),
		Status:         TRYBOT_STATUS_STARTED,
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"category\":\"cq\"}",
	}
	tryResult, err = TryResultFromBuildbucket(retry)
	assert.NoError(t, err)
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

	// Verify that an "experimental" trybot doesn't count against us.
	exp := &buildbucket.Build{
		Created:        jsonutils.Time(time.Now().UTC()),
		Result:         TRYBOT_RESULT_SUCCESS,
		Status:         TRYBOT_STATUS_COMPLETED,
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"category\":\"cq-experimental\"}",
	}
	tryResult, err = TryResultFromBuildbucket(exp)
	assert.NoError(t, err)
	roll.TryResults = append(roll.TryResults, tryResult)
	assert.True(t, roll.AllTrybotsFinished())
	assert.True(t, roll.AllTrybotsSucceeded())
}
