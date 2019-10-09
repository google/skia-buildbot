package buildbucket_test

import (
	"context"
	"testing"

	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/buildbucket/bb_testutils"
	"go.skia.org/infra/go/deepequal"
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
	deepequal.AssertDeepEqual(t, expect, b)
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
	}, []*buildbucketpb.Build{expect}, nil)
	b, err := c.GetTrybotsForCL(context.TODO(), 12345, 1, "https://skia-review.googlesource.com")
	require.NoError(t, err)
	require.NotNil(t, b)
	require.Equal(t, 1, len(b))
	deepequal.AssertDeepEqual(t, expect, b[0])
}
