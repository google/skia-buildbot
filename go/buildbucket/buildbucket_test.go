package buildbucket_test

import (
	"context"
	"testing"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/buildbucket/bb_testutils"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGetBuild(t *testing.T) {
	unittest.SmallTest(t)

	id := int64(12345)
	c := bb_testutils.NewMockClient(t)

	expect := &buildbucketpb.Build{
		Builder: &buildbucketpb.BuilderID{
			Project: "fake",
			Bucket:  "skia.primary",
			Builder: "Housekeeper-OnDemand-Presubmit",
		},
		EndTime: &timestamp.Timestamp{
			Seconds: 1553793030,
			Nanos:   570629000,
		},
		CreatedBy: "some@user.com",
		CreateTime: &timestamp.Timestamp{
			Seconds: 1553792903,
			Nanos:   783203000,
		},
		Id: id,
		Input: &buildbucketpb.Build_Input{
			GerritChanges: []*buildbucketpb.GerritChange{
				{
					Host:     "skia-review.googlesource.com",
					Project:  "skia",
					Change:   12345,
					Patchset: 1,
				},
			},
		},
		Status: buildbucketpb.Status_SUCCESS,
	}
	c.MockGetBuild(expect.Id, expect, nil)
	b, err := c.GetBuild(context.TODO(), id)
	require.NoError(t, err)
	require.NotNil(t, b)
	assertdeep.Equal(t, expect, b)
}

func TestScheduleBuilds(t *testing.T) {
	unittest.SmallTest(t)

	buildID := int64(12345)
	bbProject := "fake"
	bbBucket := "skia.primary"
	builderName := "Housekeeper-OnDemand-Presubmit"
	gerritHost := "skia-review.googlesource.com"
	repo := "skia"
	change := int64(32423)
	patchset := int64(1)
	tagName := "test-tag-name"
	tagValue := "test-tag-value"

	c := bb_testutils.NewMockClient(t)

	expectBuild := &buildbucketpb.Build{
		Builder: &buildbucketpb.BuilderID{
			Project: bbProject,
			Bucket:  bbBucket,
			Builder: builderName,
		},
		EndTime: &timestamp.Timestamp{
			Seconds: 1553793030,
			Nanos:   570629000,
		},
		CreatedBy: "some@user.com",
		CreateTime: &timestamp.Timestamp{
			Seconds: 1553792903,
			Nanos:   783203000,
		},
		Id: buildID,
		Input: &buildbucketpb.Build_Input{
			GerritChanges: []*buildbucketpb.GerritChange{
				{
					Host:     gerritHost,
					Project:  repo,
					Change:   change,
					Patchset: patchset,
				},
			},
		},
		Status: buildbucketpb.Status_SUCCESS,
	}
	expectResponse := &buildbucketpb.BatchResponse{
		Responses: []*buildbucketpb.BatchResponse_Response{
			{
				Response: &buildbucketpb.BatchResponse_Response_ScheduleBuild{
					ScheduleBuild: expectBuild,
				},
			},
		},
	}

	c.MockScheduleBuilds(builderName, tagName, tagValue, gerritHost, repo, bbProject, bbBucket, change, patchset, expectResponse, nil)
	builds, err := c.ScheduleBuilds(context.TODO(), []string{builderName}, map[string]map[string]string{builderName: {tagName: tagValue}}, change, patchset, gerritHost, repo, bbProject, bbBucket)
	require.NoError(t, err)
	require.NotNil(t, builds)
	require.Equal(t, 1, len(builds))
	assertdeep.Equal(t, expectBuild, builds[0])
}

func TestCancelBuilds(t *testing.T) {
	unittest.SmallTest(t)

	buildID := int64(12345)
	bbProject := "fake"
	bbBucket := "skia.primary"
	builderName := "Housekeeper-OnDemand-Presubmit"
	gerritHost := "skia-review.googlesource.com"
	repo := "skia"
	change := int64(32423)
	patchset := int64(1)
	summaryMarkdown := "Cancelling for testing reasons"

	c := bb_testutils.NewMockClient(t)

	expectBuild := &buildbucketpb.Build{
		Builder: &buildbucketpb.BuilderID{
			Project: bbProject,
			Bucket:  bbBucket,
			Builder: builderName,
		},
		EndTime: &timestamp.Timestamp{
			Seconds: 1553793030,
			Nanos:   570629000,
		},
		CreatedBy: "some@user.com",
		CreateTime: &timestamp.Timestamp{
			Seconds: 1553792903,
			Nanos:   783203000,
		},
		Id: buildID,
		Input: &buildbucketpb.Build_Input{
			GerritChanges: []*buildbucketpb.GerritChange{
				{
					Host:     gerritHost,
					Project:  repo,
					Change:   change,
					Patchset: patchset,
				},
			},
		},
		Status: buildbucketpb.Status_SUCCESS,
	}
	expectResponse := &buildbucketpb.BatchResponse{
		Responses: []*buildbucketpb.BatchResponse_Response{
			{
				Response: &buildbucketpb.BatchResponse_Response_CancelBuild{
					CancelBuild: expectBuild,
				},
			},
		},
	}

	c.MockCancelBuilds(buildID, summaryMarkdown, expectResponse, nil)
	builds, err := c.CancelBuilds(context.TODO(), []int64{buildID}, summaryMarkdown)
	require.NoError(t, err)
	require.NotNil(t, builds)
	require.Equal(t, 1, len(builds))
	assertdeep.Equal(t, expectBuild, builds[0])
}

func TestGetTrybotsForCL(t *testing.T) {
	unittest.SmallTest(t)

	id := int64(12345)
	c := bb_testutils.NewMockClient(t)

	expect := &buildbucketpb.Build{
		Builder: &buildbucketpb.BuilderID{
			Project: "skia",
			Bucket:  "skia.primary",
			Builder: "Housekeeper-OnDemand-Presubmit",
		},
		EndTime: &timestamp.Timestamp{
			Seconds: 1553793030,
			Nanos:   570629000,
		},
		CreatedBy: "some@user.com",
		CreateTime: &timestamp.Timestamp{
			Seconds: 1553792903,
			Nanos:   783203000,
		},
		Id: id,
		Input: &buildbucketpb.Build_Input{
			GerritChanges: []*buildbucketpb.GerritChange{
				{
					Host:     "skia-review.googlesource.com",
					Project:  "skia",
					Change:   12345,
					Patchset: 1,
				},
			},
		},
		Status: buildbucketpb.Status_SUCCESS,
	}
	c.MockSearchBuilds(&buildbucketpb.BuildPredicate{
		GerritChanges: []*buildbucketpb.GerritChange{
			{
				Host:     "skia-review.googlesource.com",
				Change:   12345,
				Patchset: 1,
			},
		},
		Tags: []*buildbucketpb.StringPair{},
	}, []*buildbucketpb.Build{expect}, nil)
	b, err := c.GetTrybotsForCL(context.TODO(), 12345, 1, "https://skia-review.googlesource.com", nil)
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, 1, len(b))
	assertdeep.Equal(t, expect, b[0])
}
