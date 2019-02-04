package bbstate

import (
	"context"
	"net/http"
	"regexp"
	"time"

	"github.com/golang/protobuf/ptypes/timestamp"
	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	"go.chromium.org/luci/grpc/prpc"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/tryjobstore"
	"google.golang.org/genproto/protobuf/field_mask"
)

var buildFieldMask = &field_mask.FieldMask{
	Paths: []string{
		"id",
		"builder",
		"number",
		"created_by",
		"create_time",
		"start_time",
		"end_time",
		"update_time",
		"status",
		"input",
		"output",
		"steps",
		"infra",
		"tags",
	},
}

type bbServiceV2 struct {
	client        buildbucketpb.BuildsClient
	bucketName    string
	builderRegExp *regexp.Regexp
}

func newBuildBucketV2(httpClient *http.Client, buildBucketURL, bucketName string, builderRegExp *regexp.Regexp) (iBuildBucketSvc, error) {
	// ctx := context.TODO()
	// func newBuildsClient(c context.Context, host string) (buildbucketpb.BuildsClient, error) {
	// t, err := auth.GetRPCTransport(c, auth.AsSelf)
	// if err != nil {
	// 	return nil, err
	// }
	client := buildbucketpb.NewBuildsPRPCClient(&prpc.Client{
		C:    httpClient,
		Host: buildBucketURL,
	})

	return &bbServiceV2{
		client:        client,
		bucketName:    bucketName,
		builderRegExp: builderRegExp,
	}, nil
}

func (b *bbServiceV2) Get(buildBucketID int64) (*tryjobstore.Tryjob, error) {
	ctx := context.TODO()
	buildResp, err := b.client.GetBuild(ctx, &buildbucketpb.GetBuildRequest{
		Id:     buildBucketID,
		Fields: buildFieldMask,
	})
	if err != nil {
		return nil, err
	}

	sklog.Infof("DATA:\n %d     %d   %s \n", buildResp.GetStatus(), buildResp.Id, buildResp.Builder.GetBuilder())
	if true {
		panic("Not implemented yet")
	}
	return nil, nil
}

func (b *bbServiceV2) Search(resultCh chan<- *tBuildInfo, timeWindow time.Duration) error {
	ctx := context.TODO()
	pageToken := ""

	timeWindowStartNs := time.Now().Add(-timeWindow).UnixNano()
	// searchCall.Bucket(b.bucketName).CreationTsLow(timeWindowStart)
	tsStart := &timestamp.Timestamp{
		Seconds: timeWindowStartNs / int64(time.Second),
		Nanos:   int32(timeWindowStartNs % int64(time.Second)),
	}
	req := &buildbucketpb.SearchBuildsRequest{
		Predicate: &buildbucketpb.BuildPredicate{
			CreateTime: &buildbucketpb.TimeRange{StartTime: tsStart},
			Builder: &buildbucketpb.BuilderID{
				Project: "chromium",
				Bucket:  b.bucketName,
			},
		},
		PageToken: pageToken,
	}

	for {
		resp, err := b.client.SearchBuilds(ctx, req)
		if err != nil {
			return err
		}

		for _, build := range resp.Builds {
			bi := &tBuildInfo{Id: build.Id}
			switch build.GetStatus() {
			case buildbucketpb.Status_SCHEDULED:
				bi.Status = bbs_STATUS_SCHEDULED
			case buildbucketpb.Status_STARTED:
				bi.Status = bbs_STATUS_STARTED
			default:
				bi.Status = bbs_STATUS_OTHER
			}
			resultCh <- bi
		}

		pageToken := resp.GetNextPageToken()
		if pageToken == "" {
			break
		}
		req.PageToken = pageToken
	}
	return nil
}
