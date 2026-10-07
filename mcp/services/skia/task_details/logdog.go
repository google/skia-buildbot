package task_details

import (
	"context"
	"fmt"

	"go.chromium.org/luci/grpc/prpc"
	"go.chromium.org/luci/logdog/api/logpb"
	"go.chromium.org/luci/logdog/client/coordinator"
	"go.chromium.org/luci/logdog/common/types"
	annopb "go.chromium.org/luci/luciexe/legacy/annotee/proto"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2"
	"google.golang.org/protobuf/proto"
)

type LogDogClient interface {
	FetchLogEntries(ctx context.Context, project, logPath string, startIndex, limit int) ([]*logpb.LogEntry, types.MessageIndex, error)
	GetLastEntry(ctx context.Context, project, logPath string) (*logpb.LogEntry, bool, error)
	GetBuildSteps(ctx context.Context, project, taskID string) (*annopb.Step, bool, error)
}

// NewLogDogClient returns a LogDogClient backed by the LogDog coordinator.
func NewLogDogClient(ts oauth2.TokenSource) LogDogClient {
	c := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	prpcClient := prpc.Client{
		C:       c,
		Host:    logdogHost,
		Options: prpc.DefaultOptions(),
	}
	coord := coordinator.NewClient(&prpcClient)
	return &logDogClientImpl{logdog: coord}
}

type logDogClientImpl struct {
	logdog *coordinator.Client
}

func (c *logDogClientImpl) FetchLogEntries(ctx context.Context, project, logPath string, startIndex, limit int) ([]*logpb.LogEntry, types.MessageIndex, error) {
	var state coordinator.LogStream
	entries, err := c.logdog.Stream(project, types.StreamPath(logPath)).Get(ctx, coordinator.WithState(&state), coordinator.Index(types.MessageIndex(startIndex)), coordinator.LimitCount(limit))
	if err != nil {
		return nil, -1, err
	}
	return entries, state.State.TerminalIndex, nil
}

func (c *logDogClientImpl) GetLastEntry(ctx context.Context, project, logPath string) (*logpb.LogEntry, bool, error) {
	var state coordinator.LogStream
	le, err := c.logdog.Stream(project, types.StreamPath(logPath)).Tail(ctx, coordinator.WithState(&state), coordinator.Complete())
	if err != nil {
		return nil, false, err
	}
	finished := state.State.Archived || (state.State.TerminalIndex >= 0 && le != nil && types.MessageIndex(le.StreamIndex) >= state.State.TerminalIndex)
	return le, finished, nil
}

func (c *logDogClientImpl) GetBuildSteps(ctx context.Context, project, taskID string) (*annopb.Step, bool, error) {
	path := fmt.Sprintf(logdogPathTmplRun, taskID)
	stream := c.logdog.Stream(project, types.StreamPath(path))
	var state coordinator.LogStream
	le, err := stream.Tail(ctx, coordinator.WithState(&state), coordinator.Complete())
	if err != nil {
		return nil, false, skerr.Wrapf(err, "failed to tail stream")
	}
	if le == nil {
		return nil, false, skerr.Fmt("no annotation entries found in stream")
	}

	if state.Desc.ContentType != annopb.ContentTypeAnnotations {
		return nil, false, skerr.Fmt("expected annotations but found %s", state.Desc.ContentType)
	}
	dg := le.GetDatagram()
	if dg == nil {
		return nil, false, skerr.Fmt("no datagram found for step!")
	}
	var step annopb.Step
	if err := proto.Unmarshal(dg.Data, &step); err != nil {
		return nil, false, skerr.Wrapf(err, "failed to unmarshal datagram data")
	}
	finished := state.State.Archived || (state.State.TerminalIndex >= 0 && types.MessageIndex(le.StreamIndex) >= state.State.TerminalIndex)
	return &step, finished, nil
}

var _ LogDogClient = &logDogClientImpl{}
