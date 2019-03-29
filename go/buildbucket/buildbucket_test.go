package buildbucket

import (
	"context"
	fmt "fmt"
	"testing"
	"time"

	assert "github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/deepequal"
	"go.skia.org/infra/go/testutils"
)

func TestGetBuild(t *testing.T) {
	testutils.SmallTest(t)

	id := int64(12345)
	c := NewMockClient(t)

	expect := &Build{
		Bucket:    "skia.primary",
		Completed: time.Unix(1553793030, 570629000).UTC(),
		CreatedBy: "some@user.com",
		Created:   time.Unix(1553792903, 783203000).UTC(),
		Id:        fmt.Sprintf("%d", id),
		Url:       fmt.Sprintf(BUILD_URL_TMPL, apiUrl, id),
		Parameters: &Parameters{
			BuilderName: "Housekeeper-OnDemand-Presubmit",
			Properties: Properties{
				Category:       "cq",
				Gerrit:         "https://skia-review.googlesource.com",
				GerritIssue:    12345,
				GerritPatchset: "1",
				PatchProject:   "skia",
				PatchStorage:   "gerrit",
				Reason:         "CQ",
				Revision:       "HEAD",
				TryJobRepo:     "https://skia.googlesource.com/skia_internal.git",
			},
		},
		Result: RESULT_SUCCESS,
		Status: STATUS_COMPLETED,
	}
	c.MockGetBuild(expect.Id, expect, nil)
	b, err := c.GetBuild(context.TODO(), fmt.Sprintf("%d", id))
	assert.NoError(t, err)
	assert.NotNil(t, b)
	deepequal.AssertCopy(t, expect, b)
	deepequal.AssertCopy(t, expect.Parameters, b.Parameters)
	deepequal.AssertCopy(t, expect.Parameters.Properties, b.Parameters.Properties)
}

func TestGetTrybotsForCL(t *testing.T) {
	testutils.SmallTest(t)

	id := int64(12345)
	c := NewMockClient(t)

	expect := &Build{
		Bucket:    "skia.primary",
		Completed: time.Unix(1553793030, 570629000).UTC(),
		CreatedBy: "some@user.com",
		Created:   time.Unix(1553792903, 783203000).UTC(),
		Id:        fmt.Sprintf("%d", id),
		Url:       fmt.Sprintf(BUILD_URL_TMPL, apiUrl, id),
		Parameters: &Parameters{
			BuilderName: "Housekeeper-OnDemand-Presubmit",
			Properties: Properties{
				Category:       "cq",
				Gerrit:         "https://skia-review.googlesource.com",
				GerritIssue:    12345,
				GerritPatchset: "1",
				PatchProject:   "skia",
				PatchStorage:   "gerrit",
				Reason:         "CQ",
				Revision:       "HEAD",
				TryJobRepo:     "https://skia.googlesource.com/skia_internal.git",
			},
		},
		Result: RESULT_SUCCESS,
		Status: STATUS_COMPLETED,
	}
	c.MockSearchBuilds(&buildbucketpb.BuildPredicate{
		GerritChanges: []*buildbucketpb.GerritChange{
			&buildbucketpb.GerritChange{
				Host:     "skia-review.googlesource.com",
				Change:   12345,
				Patchset: 1,
			},
		},
	}, []*Build{expect}, nil)
	b, err := c.GetTrybotsForCL(context.TODO(), 12345, 1, "https://skia-review.googlesource.com")
	assert.NoError(t, err)
	assert.NotNil(t, b)
	assert.Equal(t, 1, len(b))
	deepequal.AssertCopy(t, expect, b[0])
	deepequal.AssertCopy(t, expect.Parameters, b[0].Parameters)
	deepequal.AssertCopy(t, expect.Parameters.Properties, b[0].Parameters.Properties)
}
