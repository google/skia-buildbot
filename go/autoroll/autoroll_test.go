package autoroll

import (
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/comment"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestAutoRollIssueCopy(t *testing.T) {
	unittest.SmallTest(t)
	roll := &AutoRollIssue{
		Closed: true,
		Comments: []*comment.Comment{
			{
				Id:        "123",
				Message:   "hello world",
				Timestamp: time.Now(),
				User:      "me@google.com",
			},
		},
		Committed:      true,
		Created:        time.Now(),
		CqFinished:     true,
		CqSuccess:      true,
		DryRunFinished: true,
		DryRunSuccess:  true,
		IsDryRun:       true,
		Issue:          123,
		Modified:       time.Now(),
		Patchsets:      []int64{1},
		Result:         ROLL_RESULT_SUCCESS,
		RollingFrom:    "abc123",
		RollingTo:      "def456",
		Subject:        "Roll src/third_party/skia abc123..def456 (3 commits).",
		TryResults: []*TryResult{
			{
				Builder:  "build",
				Category: "cats",
				Created:  time.Now(),
				Result:   TRYBOT_RESULT_SUCCESS,
				Status:   TRYBOT_STATUS_COMPLETED,
				Url:      "http://build/cats",
			},
		},
	}
	deepequal.AssertCopy(t, roll, roll.Copy())
}

func ts(t time.Time) *timestamp.Timestamp {
	rv, err := ptypes.TimestampProto(t)
	if err != nil {
		panic(err)
	}
	return rv
}

func TestTrybotResults(t *testing.T) {
	unittest.SmallTest(t)
	// Create a fake roll with one in-progress trybot.
	roll := &AutoRollIssue{
		Closed:    false,
		Committed: false,
		Created:   time.Now(),
		Issue:     123,
		Modified:  time.Now(),
		Patchsets: []int64{1},
		Subject:   "Roll src/third_party/skia abc123..def456 (3 commits).",
	}
	roll.Result = RollResult(roll)

	trybot := &buildbucketpb.Build{
		Builder: &buildbucketpb.BuilderID{
			Project: "skia",
			Bucket:  "fake",
			Builder: "fake-builder",
		},
		CreateTime: ts(time.Now().UTC()),
		Status:     buildbucketpb.Status_STARTED,
		Tags: []*buildbucketpb.StringPair{
			{
				Key:   "user_agent",
				Value: "cq",
			},
			{
				Key:   "cq_experimental",
				Value: "false",
			},
		},
	}
	tryResult, err := TryResultFromBuildbucket(trybot)
	require.NoError(t, err)
	roll.TryResults = []*TryResult{tryResult}
	require.False(t, roll.AllTrybotsFinished())
	require.False(t, roll.AllTrybotsSucceeded())

	// Trybot failed.
	tryResult.Status = TRYBOT_STATUS_COMPLETED
	tryResult.Result = TRYBOT_RESULT_FAILURE
	require.True(t, roll.AllTrybotsFinished())
	require.False(t, roll.AllTrybotsSucceeded())

	retry := &buildbucketpb.Build{
		Builder: &buildbucketpb.BuilderID{
			Project: "skia",
			Bucket:  "fake",
			Builder: "fake-builder",
		},
		CreateTime: ts(time.Now().UTC()),
		Status:     buildbucketpb.Status_STARTED,
		Tags: []*buildbucketpb.StringPair{
			{
				Key:   "user_agent",
				Value: "cq",
			},
			{
				Key:   "cq_experimental",
				Value: "false",
			},
		},
	}
	tryResult, err = TryResultFromBuildbucket(retry)
	require.NoError(t, err)
	roll.TryResults = append(roll.TryResults, tryResult)
	require.False(t, roll.AllTrybotsFinished())
	require.False(t, roll.AllTrybotsSucceeded())

	// The second try result, a retry of the first, succeeded.
	tryResult.Status = TRYBOT_STATUS_COMPLETED
	tryResult.Result = TRYBOT_RESULT_SUCCESS
	require.True(t, roll.AllTrybotsFinished())
	require.True(t, roll.AllTrybotsSucceeded())

	// Verify that the ordering of try results does not matter.
	roll.TryResults[0], roll.TryResults[1] = roll.TryResults[1], roll.TryResults[0]
	require.True(t, roll.AllTrybotsFinished())
	require.True(t, roll.AllTrybotsSucceeded())

	// Verify that an "experimental" trybot doesn't count against us.
	exp := &buildbucketpb.Build{
		Builder: &buildbucketpb.BuilderID{
			Project: "skia",
			Bucket:  "fake",
			Builder: "fake-builder",
		},
		CreateTime: ts(time.Now().UTC()),
		Status:     buildbucketpb.Status_STARTED,
		Tags: []*buildbucketpb.StringPair{
			{
				Key:   "user_agent",
				Value: "cq",
			},
			{
				Key:   "cq_experimental",
				Value: "true",
			},
		},
	}
	tryResult, err = TryResultFromBuildbucket(exp)
	require.NoError(t, err)
	roll.TryResults = append(roll.TryResults, tryResult)
	require.True(t, roll.AllTrybotsFinished())
	require.True(t, roll.AllTrybotsSucceeded())
}
