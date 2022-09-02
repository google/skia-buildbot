package autoroll

import (
	"testing"
	"time"

	"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/comment"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/github"
)

func TestAutoRollIssueCopy(t *testing.T) {
	roll := &AutoRollIssue{
		Attempt: 2,
		Closed:  true,
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
	assertdeep.Copy(t, roll, roll.Copy())
}

func ts(t time.Time) *timestamp.Timestamp {
	rv, err := ptypes.TimestampProto(t)
	if err != nil {
		panic(err)
	}
	return rv
}

func TestTrybotResults(t *testing.T) {
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

func TestTryResultsFromGithubChecks(t *testing.T) {

	// Create local vars since you cannot take address of a const.
	pendingState := github.CHECK_STATE_PENDING
	failureState := github.CHECK_STATE_FAILURE
	errorState := github.CHECK_STATE_ERROR
	successState := github.CHECK_STATE_SUCCESS
	id1 := int64(1)
	id2 := int64(2)
	id3 := int64(3)
	id4 := int64(4)
	context1 := "Check1"
	context2 := "Check2"
	context3 := "Check3"
	context4 := "Check4"
	checks := []*github.Check{
		// Pending check.
		{ID: id1, Name: context1, State: pendingState},
		// Failed check.
		{ID: id2, Name: context2, State: failureState},
		// Error check.
		{ID: id3, Name: context3, State: errorState},
		// Success check.
		{ID: id4, Name: context4, State: successState},
	}

	// Assert all try results.
	tryResults := TryResultsFromGithubChecks(checks, []string{})
	require.True(t, len(tryResults) == 4)

	require.Equal(t, "Check1 #1", tryResults[0].Builder)
	require.Equal(t, "cq", tryResults[0].Category)
	require.Equal(t, "", tryResults[0].Result)
	require.Equal(t, TRYBOT_STATUS_STARTED, tryResults[0].Status)

	require.Equal(t, "Check2 #2", tryResults[1].Builder)
	require.Equal(t, "cq", tryResults[1].Category)
	require.Equal(t, TRYBOT_RESULT_FAILURE, tryResults[1].Result)
	require.Equal(t, TRYBOT_STATUS_COMPLETED, tryResults[1].Status)

	require.Equal(t, "Check3 #3", tryResults[2].Builder)
	require.Equal(t, "cq", tryResults[2].Category)
	require.Equal(t, TRYBOT_RESULT_FAILURE, tryResults[2].Result)
	require.Equal(t, TRYBOT_STATUS_COMPLETED, tryResults[2].Status)

	require.Equal(t, "Check4 #4", tryResults[3].Builder)
	require.Equal(t, "cq", tryResults[3].Category)
	require.Equal(t, TRYBOT_RESULT_SUCCESS, tryResults[3].Result)
	require.Equal(t, TRYBOT_STATUS_COMPLETED, tryResults[3].Status)

	// Specify a check to wait for and assert.
	tryResults = TryResultsFromGithubChecks(checks, []string{context3})
	require.True(t, len(tryResults) == 4)

	require.Equal(t, "Check3 #3", tryResults[2].Builder)
	require.Equal(t, "cq", tryResults[2].Category)
	require.Equal(t, "", tryResults[2].Result)
	require.Equal(t, TRYBOT_STATUS_STARTED, tryResults[2].Status)
}
