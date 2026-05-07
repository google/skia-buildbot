package task_details

import (
	"context"
	"fmt"

	"go.chromium.org/luci/logdog/api/logpb"
	"go.chromium.org/luci/logdog/client/coordinator"
	"go.chromium.org/luci/logdog/common/types"
	annopb "go.chromium.org/luci/luciexe/legacy/annotee/proto"
	"go.skia.org/infra/go/skerr"
	"google.golang.org/protobuf/proto"
)

type LogDogClient interface {
	FetchLogEntries(ctx context.Context, project, logPath string, startIndex, limit int) ([]*logpb.LogEntry, error)
	GetLastEntry(ctx context.Context, project, logPath string) (*logpb.LogEntry, error)
	GetBuildSteps(ctx context.Context, project, taskID string) (*annopb.Step, error)
}

type logDogClientImpl struct {
	logdog *coordinator.Client
}

func (c *logDogClientImpl) FetchLogEntries(ctx context.Context, project, logPath string, startIndex, limit int) ([]*logpb.LogEntry, error) {
	return c.logdog.Stream(logdogProject, types.StreamPath(logPath)).Get(ctx, coordinator.Index(types.MessageIndex(startIndex)), coordinator.LimitCount(limit))
}

func (c *logDogClientImpl) GetLastEntry(ctx context.Context, project, logPath string) (*logpb.LogEntry, error) {
	return c.logdog.Stream(logdogProject, types.StreamPath(logPath)).Tail(ctx)
}

func (c *logDogClientImpl) GetBuildSteps(ctx context.Context, project, taskID string) (*annopb.Step, error) {
	path := fmt.Sprintf(logdogPathTmplRun, taskID)
	stream := c.logdog.Stream(logdogProject, types.StreamPath(path))
	var state coordinator.LogStream
	le, err := stream.Tail(ctx, coordinator.WithState(&state), coordinator.Complete())
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to tail stream")
	}
	if le == nil {
		return nil, skerr.Fmt("no annotation entries found in stream")
	}

	if state.Desc.ContentType != annopb.ContentTypeAnnotations {
		return nil, skerr.Fmt("expected annotations but found %s", state.Desc.ContentType)
	}
	dg := le.GetDatagram()
	if dg == nil {
		return nil, skerr.Fmt("no datagram found for step!")
	}
	var step annopb.Step
	if err := proto.Unmarshal(dg.Data, &step); err != nil {
		return nil, skerr.Wrapf(err, "failed to unmarshal datagram data")
	}
	return &step, nil
}

var _ LogDogClient = &logDogClientImpl{}
