package run_benchmark

import (
	"context"
	"testing"

	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	. "go.chromium.org/luci/common/testing/assertions"
	"go.skia.org/infra/bisection/go/bot_configs"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming/mocks"
)

var req = RunBenchmarkRequest{
	JobID:     "id",
	Benchmark: "benchmark",
	Story:     "story",
	Build: &swarmingV1.SwarmingRpcsCASReference{
		CasInstance: "instance",
		Digest: &swarmingV1.SwarmingRpcsDigest{
			Hash:      "hash",
			SizeBytes: 0,
		},
	},
	Commit: "64893ca6294946163615dcf23b614afe0419bfa3",
}
var expectedErr = skerr.Fmt("some error")

func TestListPinpointTasks(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)

	Convey(`OK`, t, func() {
		Convey(`Tasks found`, func() {
			mockClient.On("ListTasks", ctx, mock.Anything, mock.Anything,
				mock.Anything, mock.Anything).
				Return([]*swarmingV1.SwarmingRpcsTaskRequestMetadata{
					{
						TaskId: "123",
					},
					{
						TaskId: "456",
					},
				}, nil).Once()
			taskIds, err := ListPinpointTasks(ctx, mockClient, req)
			So(err, ShouldBeNil)
			So(taskIds, ShouldEqual, []string{"123", "456"})
		})
		Convey(`No tasks found`, func() {
			mockClient.On("ListTasks", ctx, mock.Anything, mock.Anything,
				mock.Anything, mock.Anything).
				Return([]*swarmingV1.SwarmingRpcsTaskRequestMetadata{}, nil).Once()
			taskIds, err := ListPinpointTasks(ctx, mockClient, req)
			So(err, ShouldBeNil)
			So(taskIds, ShouldBeEmpty)
		})
	})
	Convey(`Return error`, t, func() {
		Convey(`Missing inputs`, func() {
			req := RunBenchmarkRequest{}
			taskIds, err := ListPinpointTasks(ctx, mockClient, req)
			So(taskIds, ShouldBeNil)
			So(err, ShouldErrLike, "Cannot list tasks because request is missing JobID")
			req.JobID = "1"
			taskIds, err = ListPinpointTasks(ctx, mockClient, req)
			So(taskIds, ShouldBeNil)
			So(err, ShouldErrLike, "Cannot list tasks because request is missing cas isolate")
		})
		Convey(`Client failure`, func() {
			mockClient.On("ListTasks", ctx, mock.Anything, mock.Anything,
				mock.Anything, mock.Anything).
				Return([]*swarmingV1.SwarmingRpcsTaskRequestMetadata{}, expectedErr).Once()
			taskIds, err := ListPinpointTasks(ctx, mockClient, req)
			So(taskIds, ShouldBeNil)
			So(err, ShouldErrLike, expectedErr)
		})
	})
}

func TestGetCasOutput(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)

	Convey(`OK`, t, func() {
		Convey(`CAS found`, func() {
			mockClient.On("GetTask", ctx, mock.Anything, mock.Anything).
				Return(&swarmingV1.SwarmingRpcsTaskResult{
					State: "COMPLETED",
					CasOutputRoot: &swarmingV1.SwarmingRpcsCASReference{
						CasInstance: "instance",
						Digest: &swarmingV1.SwarmingRpcsDigest{
							Hash:      "hash",
							SizeBytes: 0,
						},
					},
				}, nil).Once()
			rbe, err := GetCASOutput(ctx, mockClient, "taskId")
			So(err, ShouldBeNil)
			So(rbe.CasInstance, ShouldEqual, "instance")
			So(rbe.Digest.Hash, ShouldEqual, "hash")
			So(rbe.Digest.SizeBytes, ShouldEqual, 0)
		})
	})
	Convey(`Return error`, t, func() {
		Convey(`Task not completed`, func() {
			mockClient.On("GetTask", ctx, mock.Anything, mock.Anything).
				Return(&swarmingV1.SwarmingRpcsTaskResult{
					State: "Not_Completed",
				}, nil).Once()
			rbe, err := GetCASOutput(ctx, mockClient, "taskId")
			So(err, ShouldErrLike, "cannot get result of task")
			So(rbe, ShouldBeNil)
		})
		Convey(`Client failure`, func() {
			mockClient.On("GetTask", ctx, mock.Anything, mock.Anything).
				Return(nil, expectedErr).Once()
			taskIds, err := GetCASOutput(ctx, mockClient, "taskId")
			So(taskIds, ShouldBeNil)
			So(err, ShouldErrLike, expectedErr)
		})
	})
}

func TestRun(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)

	Convey(`OK`, t, func() {
		cfg, err := bot_configs.GetBotConfig("linux-perf", true)
		So(err, ShouldBeNil)
		req.Config = cfg
		mockClient.On("TriggerTask", ctx, mock.Anything).
			Return(&swarmingV1.SwarmingRpcsTaskRequestMetadata{
				TaskId: "123",
			}, nil).Once()
		taskId, err := Run(ctx, mockClient, req)
		So(err, ShouldBeNil)
		So(taskId, ShouldEqual, "123")
	})
	Convey(`Return error`, t, func() {
		cfg, err := bot_configs.GetBotConfig("linux-perf", true)
		So(err, ShouldBeNil)
		req.Config = cfg
		mockClient.On("TriggerTask", ctx, mock.Anything).
			Return(&swarmingV1.SwarmingRpcsTaskRequestMetadata{
				TaskId: "123",
			}, expectedErr).Once()
		taskId, err := Run(ctx, mockClient, req)
		So(taskId, ShouldBeEmpty)
		So(err, ShouldErrLike, expectedErr)
	})
}
