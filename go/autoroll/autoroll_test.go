package autoroll

import (
	"context"
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
	from, to, err := RollRev(context.Background(), roll.Subject, func(ctx context.Context, h string) (string, error) {
		return h, nil
	})
	assert.NoError(t, err)
	roll.RollingFrom = from
	roll.RollingTo = to

	trybot := &buildbucket.Build{
		Created:        jsonutils.Time(time.Now().UTC()),
		Status:         TRYBOT_STATUS_STARTED,
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"properties\":{\"category\":\"cq\"}}",
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
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"properties\":{\"category\":\"cq\"}}",
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
		ParametersJson: "{\"builder_name\":\"fake-builder\",\"properties\":{\"category\":\"cq-experimental\"}}",
	}
	tryResult, err = TryResultFromBuildbucket(exp)
	assert.NoError(t, err)
	roll.TryResults = append(roll.TryResults, tryResult)
	assert.True(t, roll.AllTrybotsFinished())
	assert.True(t, roll.AllTrybotsSucceeded())
}

func TestRollRev(t *testing.T) {
	testutils.SmallTest(t)

	test := func(msg, from, to string) {
		assert.True(t, ROLL_REV_REGEX.MatchString(msg))
		a, b, err := RollRev(context.Background(), msg, nil)
		assert.NoError(t, err)
		assert.Equal(t, from, a)
		assert.Equal(t, to, b)
	}

	test("Roll skia/third_party/externals/skcms/ 839318c8b..0e960c612 (1 commit)", "839318c8b", "0e960c612")
	test("Roll src/third_party/skia/ 2a223358e..ace53c313 (6 commits)", "2a223358e", "ace53c313")
	test("Roll src/third_party/skia/ 2a223358e..ace53c313 (6 commits).", "2a223358e", "ace53c313")
	test("Roll src/third_party/skia/ 2a223358e..ace53c313", "2a223358e", "ace53c313")
	test("Roll AFDO from 66.0.3336.3_rc-r1 to 66.0.3337.3_rc-r1", "66.0.3336.3_rc-r1", "66.0.3337.3_rc-r1")
	test("[manifest] Roll skia 1f1bb9c0b..4150eea6c (1 commits)", "1f1bb9c0b", "4150eea6c")
	test("Roll Fuchsia SDK from abc123 to def456", "abc123", "def456")
}
