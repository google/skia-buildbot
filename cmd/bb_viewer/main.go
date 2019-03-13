package main

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes"
	bb_api "go.chromium.org/luci/buildbucket/proto"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	_ "go.skia.org/infra/golden/go/tryjobstore" // Import registers event codecs in that package.
	"google.golang.org/grpc"
)

// // Command line flags.
// var (
// 	channels        = flag.String("channels", expstorage.EV_EXPSTORAGE_CHANGED, "Comma separated list of event channels.")
// 	objectPrefix    = flag.String("object_prefix", "", "Prefix of the storage path that should be watched.")
// 	objectRegExpStr = flag.String("object_regex", "", "Regex that must be matched by the object id")
// 	projectID       = flag.String("project_id", common.PROJECT_ID, "Project ID of the Cloud project where the PubSub topic lives.")
// 	storageBucket   = flag.String("bucket", "", "ID of the pubsub subscriber.")
// 	subscriberName  = flag.String("subscriber", "local-wien", "ID of the pubsub subscriber.")
// 	topic           = flag.String("topic", "testing-gold-stage-eventbus", "Google Cloud PubSub topic of the eventbus.")
// )

func main() {
	common.Init()

	// conn, err := grpc.Dial(addr,
	// 	grpc.WithInsecure(),
	// 	grpc.WithDefaultCallOptions(
	// 		grpc.MaxCallSendMsgSize(diffstore.MAX_MESSAGE_SIZE),
	// 		grpc.MaxCallRecvMsgSize(diffstore.MAX_MESSAGE_SIZE)))
	// if err != nil {
	// 	sklog.Fatalf("Unable to connect to grpc service: %s", err)
	// }

	buildBucketURL := "https://cr-buildbucket.appspot.com"
	bucketID := ""
	conn, err := grpc.Dial(buildBucketURL)
	if err != nil {
		sklog.Fatalf("Unable to connect to grpc service: %s", err)
	}

	client := bb_api.NewBuildsClient(conn)
	timeWindow := time.Hour * 24
	timeWindowStart, err := ptypes.TimestampProto(time.Now().Add(-timeWindow))
	if err != nil {
		sklog.Fatalf("Error: %s", err)
	}

	ctx := context.TODO()
	req := &bb_api.SearchBuildsRequest{
		Predicate: &bb_api.BuildPredicate{
			CreateTime: &bb_api.TimeRange{StartTime: timeWindowStart},
		},
	}
	resp, err := client.SearchBuilds(ctx, req)
	if err != nil {
		sklog.Fatalf("Error: %s", err)
	}

	for _, build := range resp.Builds {
		sklog.Infof("BUILD: %d - %s - %s", build.Id, build.Builder, build.Status)
	}
}
