package bb_testutils

import (
	"context"
	"strconv"
	"time"

	"github.com/golang/mock/gomock"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/golang/protobuf/ptypes/timestamp"
	assert "github.com/stretchr/testify/require"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/buildbucket"
	"go.skia.org/infra/go/buildbucket/common"
	"go.skia.org/infra/go/sktest"
)

var MockBBURL = "mock-buildbucket.appspot.com"

// MockClient is a wrapper around Client which doesn't actually perform API
// calls but instead simply returns mock results. Call any of the Mock* methods
// before calling the corresponding method on Client, and the mocked result will
// be returned.
type MockClient struct {
	*buildbucket.Client
	mock *buildbucketpb.MockBuildsClient
	t    sktest.TestingT
}

func NewMockClient(t sktest.TestingT) *MockClient {
	ctrl := gomock.NewController(t)
	mock := buildbucketpb.NewMockBuildsClient(ctrl)
	return &MockClient{
		Client: buildbucket.NewTestingClient(mock, MockBBURL),
		mock:   mock,
		t:      t,
	}
}

func (c *MockClient) MockGetBuild(id string, rv *buildbucket.Build, rvErr error) {
	buildId, err := strconv.ParseInt(id, 10, 64)
	assert.NoError(c.t, err)
	call := c.mock.EXPECT().GetBuild(context.TODO(), &buildbucketpb.GetBuildRequest{
		Id:     buildId,
		Fields: common.GetBuildFields,
	})
	var build *buildbucketpb.Build
	if rv != nil {
		build = unconvertBuild(c.t, rv)
	}
	call.Return(build, rvErr)
}

func (c *MockClient) MockSearchBuilds(pred *buildbucketpb.BuildPredicate, rv []*buildbucket.Build, rvErr error) {
	call := c.mock.EXPECT().SearchBuilds(context.TODO(), &buildbucketpb.SearchBuildsRequest{
		Predicate: pred,
		Fields:    common.SearchBuildsFields,
	})
	var resp *buildbucketpb.SearchBuildsResponse
	if rv != nil {
		builds := make([]*buildbucketpb.Build, 0, len(rv))
		for _, b := range rv {
			builds = append(builds, unconvertBuild(c.t, b))
		}
		resp = &buildbucketpb.SearchBuildsResponse{
			Builds: builds,
		}
	}
	call.Return(resp, rvErr)
}

func (c *MockClient) MockGetTrybotsForCL(issueID, patchsetID int64, gerritUrl string, rv []*buildbucket.Build, rvErr error) {
	pred, err := common.GetTrybotsForCLPredicate(issueID, patchsetID, gerritUrl)
	assert.NoError(c.t, err)
	c.MockSearchBuilds(pred, rv, rvErr)
}

func makeSVal(s string) *structpb.Value {
	return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: s}}
}

func makeIVal(i int64) *structpb.Value {
	return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(i)}}
}

func ts(t time.Time) *timestamp.Timestamp {
	secs := t.Unix()
	nanos := t.UnixNano() - secs*int64(time.Second)
	return &timestamp.Timestamp{
		Seconds: secs,
		Nanos:   int32(nanos),
	}
}

func unconvertBuild(t sktest.TestingT, b *buildbucket.Build) *buildbucketpb.Build {
	id, err := strconv.ParseInt(b.Id, 10, 64)
	assert.NoError(t, err)
	patchset, err := strconv.ParseInt(b.Parameters.Properties.GerritPatchset, 10, 64)
	assert.NoError(t, err)
	status := buildbucketpb.Status_STATUS_UNSPECIFIED
	switch b.Status {
	case buildbucket.STATUS_SCHEDULED:
		status = buildbucketpb.Status_SCHEDULED
	case buildbucket.STATUS_STARTED:
		status = buildbucketpb.Status_STARTED
	case buildbucket.STATUS_COMPLETED:
		switch b.Result {
		case buildbucket.RESULT_CANCELED:
			status = buildbucketpb.Status_CANCELED
		case buildbucket.RESULT_FAILURE:
			status = buildbucketpb.Status_FAILURE
		case buildbucket.RESULT_SUCCESS:
			status = buildbucketpb.Status_SUCCESS
		}
	}
	return &buildbucketpb.Build{
		Id: id,
		Builder: &buildbucketpb.BuilderID{
			Project: b.Parameters.Properties.PatchProject,
			Bucket:  b.Bucket,
			Builder: b.Parameters.BuilderName,
		},
		CreatedBy:  b.CreatedBy,
		CreateTime: ts(b.Created),
		EndTime:    ts(b.Completed),
		Status:     status,
		Input: &buildbucketpb.Build_Input{
			Properties: &structpb.Struct{
				Fields: map[string]*structpb.Value{
					"category":         makeSVal(b.Parameters.Properties.Category),
					"patch_gerrit_url": makeSVal(b.Parameters.Properties.Gerrit),
					"patch_issue":      makeIVal(b.Parameters.Properties.GerritIssue),
					"patch_project":    makeSVal(b.Parameters.Properties.PatchProject),
					"patch_set":        makeIVal(patchset),
					"patch_storage":    makeSVal(b.Parameters.Properties.PatchStorage),
					"reason":           makeSVal(b.Parameters.Properties.Reason),
					"revision":         makeSVal(b.Parameters.Properties.Revision),
					"try_job_repo":     makeSVal(b.Parameters.Properties.TryJobRepo),
				},
			},
		},
	}
}
