package bb_testutils

import (
	"context"
	"time"

	"github.com/golang/mock/gomock"
	"github.com/golang/protobuf/ptypes"
	structpb "github.com/golang/protobuf/ptypes/struct"
	"github.com/golang/protobuf/ptypes/timestamp"
	"github.com/stretchr/testify/require"
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

func (c *MockClient) MockGetBuild(id int64, rv *buildbucketpb.Build, rvErr error) {
	call := c.mock.EXPECT().GetBuild(context.TODO(), &buildbucketpb.GetBuildRequest{
		Id:     id,
		Fields: common.GetBuildFields,
	})
	call.Return(rv, rvErr)
}

func (c *MockClient) MockScheduleBuilds(b, tagName, tagValue, gerritURL, repo, bbProject, bbBucket string, issue, patchset int64, rv *buildbucketpb.BatchResponse, rvErr error) {
	call := c.mock.EXPECT().Batch(context.TODO(), &buildbucketpb.BatchRequest{
		Requests: []*buildbucketpb.BatchRequest_Request{
			{
				Request: &buildbucketpb.BatchRequest_Request_ScheduleBuild{
					ScheduleBuild: &buildbucketpb.ScheduleBuildRequest{
						Builder: &buildbucketpb.BuilderID{
							Project: bbProject,
							Bucket:  bbBucket,
							Builder: b,
						},
						GerritChanges: []*buildbucketpb.GerritChange{
							{
								Host:     gerritURL,
								Project:  repo,
								Change:   issue,
								Patchset: patchset,
							},
						},
						Properties: &structpb.Struct{},
						Tags: []*buildbucketpb.StringPair{
							{
								Key:   tagName,
								Value: tagValue,
							},
						},
						Fields: common.GetBuildFields,
					},
				},
			},
		},
	})
	call.Return(rv, rvErr)
}

func (c *MockClient) MockCancelBuilds(buildID int64, summaryMarkdown string, rv *buildbucketpb.BatchResponse, rvErr error) {
	call := c.mock.EXPECT().Batch(context.TODO(), &buildbucketpb.BatchRequest{
		Requests: []*buildbucketpb.BatchRequest_Request{
			{
				Request: &buildbucketpb.BatchRequest_Request_CancelBuild{
					CancelBuild: &buildbucketpb.CancelBuildRequest{
						Id:              buildID,
						SummaryMarkdown: summaryMarkdown,
					},
				},
			},
		},
	})
	call.Return(rv, rvErr)
}

func (c *MockClient) MockSearchBuilds(pred *buildbucketpb.BuildPredicate, rv []*buildbucketpb.Build, rvErr error) {
	call := c.mock.EXPECT().SearchBuilds(context.TODO(), &buildbucketpb.SearchBuildsRequest{
		Predicate: pred,
		Fields:    common.SearchBuildsFields,
	})
	var resp *buildbucketpb.SearchBuildsResponse
	if rv != nil {
		resp = &buildbucketpb.SearchBuildsResponse{
			Builds: rv,
		}
	}
	call.Return(resp, rvErr)
}

func (c *MockClient) MockGetTrybotsForCL(issueID, patchsetID int64, gerritUrl string, rv []*buildbucketpb.Build, rvErr error) {
	pred, err := common.GetTrybotsForCLPredicate(issueID, patchsetID, gerritUrl)
	require.NoError(c.t, err)
	c.MockSearchBuilds(pred, rv, rvErr)
}

func makeSVal(s string) *structpb.Value {
	return &structpb.Value{Kind: &structpb.Value_StringValue{StringValue: s}}
}

func makeIVal(i int64) *structpb.Value {
	return &structpb.Value{Kind: &structpb.Value_NumberValue{NumberValue: float64(i)}}
}

func ts(t time.Time) *timestamp.Timestamp {
	rv, err := ptypes.TimestampProto(t)
	if err != nil {
		panic(err)
	}
	return rv
}
