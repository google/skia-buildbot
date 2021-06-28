package verifiers

import (
	"context"
	"testing"
	"time"

	mock "github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"google.golang.org/protobuf/types/known/timestamppb"

	bb_mocks "go.skia.org/infra/go/buildbucket/mocks"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/testutils"
	cr_mocks "go.skia.org/infra/skcq/go/codereview/mocks"
	"go.skia.org/infra/skcq/go/footers"
	"go.skia.org/infra/skcq/go/types"
	"go.skia.org/infra/task_scheduler/go/specs"
)

var (
	currentTime = time.Unix(1598467386, 0).UTC()
)

func TestVerify_NoTryJobs_TasksCfg(t *testing.T) {
	tv := &TryJobsVerifier{
		tasksCfg: nil,
	}
	vs, _, err := tv.Verify(context.Background(), nil, 0)
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)

	tasksCfg := &specs.TasksCfg{}
	tv = &TryJobsVerifier{
		tasksCfg: tasksCfg,
	}
	vs, _, err = tv.Verify(context.Background(), nil, 0)
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)

	tasksCfg = &specs.TasksCfg{
		CommitQueue: map[string]*specs.CommitQueueJobConfig{},
	}
	tv = &TryJobsVerifier{
		tasksCfg: tasksCfg,
	}
	vs, _, err = tv.Verify(context.Background(), nil, 0)
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)
}

func TestVerify_NoTryJobs_Footer(t *testing.T) {
	tasksCfg := &specs.TasksCfg{
		CommitQueue: map[string]*specs.CommitQueueJobConfig{
			"try_job1": {},
		},
	}
	ci := &gerrit.ChangeInfo{
		Issue: 123,
	}

	tv := &TryJobsVerifier{
		tasksCfg: tasksCfg,
		footersMap: map[string]string{
			string(footers.NoTryFooter): "true",
		},
	}
	vs, _, err := tv.Verify(context.Background(), ci, 0)
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)
}

// cr - mock - GetLatestPatchSetID, GetEquivalentPatchSetIDs, GetFileNames (for location )
//             in cleanup - GetIssueProperties, GetEarliestEquivalentPatchSetID
//
//	latestPatchSetID := tv.cr.GetLatestPatchSetID(ci)
// equivalentPatchSetIDS := tv.cr.GetEquivalentPatchSetIDs(ci, latestPatchSetID)
//
//
// bb2 - GetTrybotsForCL, ScheduleBuilds (GetTrybotsForCL after schedule buidls is done!)
//       in cleanup - GetTrybotsForCL, CancelBuilds
// 	// bb2.GetTrybotsForCL(ctx, ci.Issue, p, "https://"+tv.gerritURL, nil)

func setupTest() (*gerrit.ChangeInfo, string, *specs.TasksCfg) {
	ci := &gerrit.ChangeInfo{
		Issue:   123,
		Project: "skia",
	}
	gerritURL := "skia-review.googlesource.com"
	tasksCfg := &specs.TasksCfg{
		CommitQueue: map[string]*specs.CommitQueueJobConfig{
			"try_job1": {},
			"try_job2": {},
		},
	}
	timeNowFunc = func() time.Time {
		return currentTime
	}
	return ci, gerritURL, tasksCfg
}

func TestVerify_AllSuccessfulTryJobs_InSamePatchSet(t *testing.T) {
	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: "try_job1"},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: "try_job2"},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, 130)
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)
}

func TestVerify_AllSuccessfulTryJobs_InEquivalentPatchSets(t *testing.T) {
	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID5 := int64(5)
	equivalentPatchsetID4 := int64(4)
	equivalentPatchsetID3 := int64(3)

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID5).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID5).Return([]int64{latestPatchsetID5, equivalentPatchsetID4, equivalentPatchsetID3}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuildPS4 := &buildbucketpb.Build{
		Builder:    &buildbucketpb.BuilderID{Builder: "try_job1"},
		CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
		Status:     buildbucketpb.Status_SUCCESS,
	}
	bbBuildPS3 := &buildbucketpb.Build{
		Builder:    &buildbucketpb.BuilderID{Builder: "try_job2"},
		CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
		Status:     buildbucketpb.Status_SUCCESS,
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID5, "https://"+gerritURL, map[string]string(nil)).Return([]*buildbucketpb.Build{}, nil).Once()
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, equivalentPatchsetID4, "https://"+gerritURL, map[string]string(nil)).Return([]*buildbucketpb.Build{bbBuildPS4}, nil).Once()
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, equivalentPatchsetID3, "https://"+gerritURL, map[string]string(nil)).Return([]*buildbucketpb.Build{bbBuildPS3}, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
	}
	vs, _, err := tv.Verify(context.Background(), ci, 130)
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)
}

func TestVerify_AllSuccessfulTryJobs_RetriggerFooter(t *testing.T) {
	ci, gerritURL, tasksCfg := setupTest()
	latestPatchsetID := int64(5)
	tryJobName1 := "try_job1"
	tryJobName2 := "try_job2"

	// Setup codereview mock.
	cr := &cr_mocks.CodeReview{}
	cr.On("GetLatestPatchSetID", ci).Return(latestPatchsetID).Once()
	cr.On("GetEquivalentPatchSetIDs", ci, latestPatchsetID).Return([]int64{latestPatchsetID}).Once()

	// Setup buildbucket mock.
	bb := &bb_mocks.BuildBucketInterface{}
	bbBuilds := []*buildbucketpb.Build{
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName1},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
		{
			Builder:    &buildbucketpb.BuilderID{Builder: tryJobName2},
			CreateTime: &timestamppb.Timestamp{Seconds: currentTime.Unix()},
			Status:     buildbucketpb.Status_SUCCESS,
		},
	}
	bb.On("GetTrybotsForCL", testutils.AnyContext, ci.Issue, latestPatchsetID, "https://"+gerritURL, map[string]string(nil)).Return(bbBuilds, nil).Twice()
	defaultTags := map[string]string{
		"triggered_by":    "skcq",
		"cq_experimental": "false",
	}
	botsToTags := map[string]map[string]string{
		tryJobName2: defaultTags,
		tryJobName1: defaultTags,
	}
	bb.On("ScheduleBuilds", testutils.AnyContext, mock.Anything, botsToTags, ci.Issue, latestPatchsetID, gerritURL, ci.Project, BuildBucketDefaultSkiaProject, BuildBucketDefaultSkiaBucket).Return(bbBuilds, nil).Once()

	// Test verify.
	tv := &TryJobsVerifier{
		bb2:       bb,
		cr:        cr,
		tasksCfg:  tasksCfg,
		gerritURL: gerritURL,
		footersMap: map[string]string{
			string(footers.RerunTryjobsFooter): "true",
		},
	}
	vs, _, err := tv.Verify(context.Background(), ci, 130)
	require.NoError(t, err)
	require.Equal(t, types.VerifierSuccessState, vs)
}

func TestVerify_IncludeTryjobsFooter(t *testing.T) {
}

func TestVerify_IncludeTryjobsFooter_UnsupportedBucket(t *testing.T) {
}

func TestVerify_ExperimentalTryJob(t *testing.T) {
}
