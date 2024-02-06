package pinpoint

import (
	"context"
	"fmt"
	"testing"

	rbeclient "github.com/bazelbuild/remote-apis-sdks/go/pkg/client"
	. "github.com/smartystreets/goconvey/convey"
	"github.com/stretchr/testify/mock"
	. "go.chromium.org/luci/common/testing/assertions"
	"go.skia.org/infra/go/mockhttpclient"
	"go.skia.org/infra/go/skerr"
	swarmingMocks "go.skia.org/infra/go/swarming/mocks"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"go.skia.org/infra/pinpoint/go/build_chrome/mocks"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/read_values"

	bpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

func defaultRunRequest() PinpointRunRequest {
	return PinpointRunRequest{
		Device:            "linux-perf",
		Benchmark:         "benchmark",
		Story:             "story",
		Chart:             "chart",
		StartCommit:       "start_commit",
		EndCommit:         "end_commit",
		AggregationMethod: read_values.Sum.AggDataMethod(),
	}
}

func TestValidateScheduleRequest(t *testing.T) {
	Convey(`OK`, t, func() {
		ctx := context.Background()
		pp, err := New(ctx)
		So(err, ShouldBeNil)
		Convey(`Given valid inputs`, func() {
			req := defaultRunRequest()
			// Run() would result in an infinite loop
			// as shouldContinue would never return false
			err := pp.validateRunRequest(req)
			So(err, ShouldBeNil)
		})
	})

	Convey(`Fails input validation`, t, func() {
		ctx := context.Background()
		pp, err := New(ctx)
		So(err, ShouldBeNil)
		Convey(`When no base or exp hash`, func() {
			req := defaultRunRequest()
			req.StartCommit = ""
			resp, err := pp.Run(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "start"))

			req = defaultRunRequest()
			req.EndCommit = ""
			resp, err = pp.Run(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "end"))
		})
		Convey(`When bad device in request`, func() {
			req := defaultRunRequest()
			req.Device = "fake-device"
			resp, err := pp.Run(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf("Device %s not allowed", req.Device))
		})
		Convey(`When missing benchmark`, func() {
			req := defaultRunRequest()
			req.Benchmark = ""
			resp, err := pp.Run(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "benchmark"))
		})
		Convey(`When missing story`, func() {
			req := defaultRunRequest()
			req.Story = ""
			resp, err := pp.Run(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "story"))
		})
		Convey(`When missing chart`, func() {
			req := defaultRunRequest()
			req.Chart = ""
			resp, err := pp.Run(ctx, req, "")
			So(resp, ShouldBeNil)
			So(err, ShouldErrLike, fmt.Sprintf(missingRequiredParamTemplate, "chart"))
		})
	})
}

func TestShouldContinue(t *testing.T) {
	Convey(`Return True`, t, func() {
		Convey(`When no builds started`, func() {
			cdl := commitDataList{
				commits: []*commitData{
					{
						commit: &midpoint.Commit{
							GitHash:       "start commit",
							RepositoryUrl: chromiumSrcGit,
						},
					},
					{
						commit: &midpoint.Commit{
							GitHash:       "end commit",
							RepositoryUrl: chromiumSrcGit,
						},
					},
				},
			}
			cont := cdl.shouldContinue()
			So(cont, ShouldEqual, true)
		})
		Convey(`When build status is not finished`, func() {
			cdl := commitDataList{
				commits: []*commitData{
					{
						build: &buildMetadata{
							buildStatus: bpb.Status_SUCCESS,
						},
					},
					{
						build: &buildMetadata{
							buildStatus: bpb.Status_STARTED,
						},
					},
				},
			}
			cont := cdl.shouldContinue()
			So(cont, ShouldEqual, true)
		})
		Convey(`When builds are finished but not tests`, func() {
			cdl := commitDataList{
				commits: []*commitData{
					{
						build: &buildMetadata{
							buildStatus: bpb.Status_SUCCESS,
						},
						tests: &testMetadata{
							isRunning: true,
						},
					},
				},
			}
			cont := cdl.shouldContinue()
			So(cont, ShouldEqual, true)
		})
	})

	Convey(`Return False when builds and tests are completed`, t, func() {
		cdl := commitDataList{
			commits: []*commitData{
				{
					build: &buildMetadata{
						buildStatus: bpb.Status_CANCELED,
					},
					tests: &testMetadata{
						isRunning: false,
					},
				},
				{
					build: &buildMetadata{
						buildStatus: bpb.Status_FAILURE,
					},
					tests: &testMetadata{
						isRunning: false,
					},
				},
			},
		}
		cont := cdl.shouldContinue()
		So(cont, ShouldEqual, false)
	})
}

func TestPollBuild(t *testing.T) {
	Convey(`OK`, t, func() {
		ctx := context.Background()
		mbc := mocks.NewBuildChromeClient(t)
		Convey(`When one build finishes`, func() {
			cdl := commitDataList{
				commits: []*commitData{
					{
						build: &buildMetadata{
							buildID: 1,
						},
					},
					{
						build: &buildMetadata{
							buildID: 2,
						},
					},
				},
			}
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_STARTED, nil).Once()
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_FAILURE, nil).Once()
			c, err := cdl.pollBuild(ctx, mbc)
			So(c.build.buildID, ShouldEqual, 2)
			So(err, ShouldBeNil)
			So(cdl.commits[0].build.buildStatus, ShouldEqual, bpb.Status_STARTED)
			So(cdl.commits[1].build.buildStatus, ShouldEqual, bpb.Status_FAILURE)
		})
		Convey(`When no builds are finished`, func() {
			cdl := commitDataList{
				commits: []*commitData{
					{
						build: &buildMetadata{
							buildID:     1,
							buildStatus: bpb.Status_SCHEDULED,
						},
					},
					{
						build: &buildMetadata{
							buildID:     2,
							buildStatus: bpb.Status_STARTED,
						},
					},
				},
			}
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_SCHEDULED, nil).Once()
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_STARTED, nil).Once()
			c, err := cdl.pollBuild(ctx, mbc)
			So(c, ShouldBeNil)
			So(err, ShouldBeNil)
		})
		Convey(`When builds are already finished`, func() {
			cdl := commitDataList{
				commits: []*commitData{
					{
						build: &buildMetadata{
							buildID:     1,
							buildStatus: bpb.Status_FAILURE,
						},
					},
					{
						build: &buildMetadata{
							buildID:     2,
							buildStatus: bpb.Status_CANCELED,
						},
					},
				},
			}
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_FAILURE, nil).Once()
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_CANCELED, nil).Once()
			c, err := cdl.pollBuild(ctx, mbc)
			So(c, ShouldBeNil)
			So(err, ShouldBeNil)
		})
		Convey(`When tests have started`, func() {
			cdl := commitDataList{
				commits: []*commitData{
					{
						build: &buildMetadata{
							buildID:     1,
							buildStatus: bpb.Status_SUCCESS,
						},
						tests: &testMetadata{
							tasks: []string{"fake-task-id"},
						},
					},
				},
			}
			c, err := cdl.pollBuild(ctx, mbc)
			So(c, ShouldBeNil)
			So(err, ShouldBeNil)
		})

	})
	Convey(`Error`, t, func() {
		Convey(`When no valid builds to poll`, func() {
			ctx := context.Background()
			mbc := mocks.NewBuildChromeClient(t)
			cdl := commitDataList{
				commits: []*commitData{
					{
						build: &buildMetadata{
							buildID: 1,
						},
					},
					{},
				},
			}
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_STARTED, nil).Once()
			c, err := cdl.pollBuild(ctx, mbc)
			So(c, ShouldBeNil)
			So(err, ShouldErrLike, "Cannot poll build of non-existent build")
		})
		Convey(`When client fails GetStatus`, func() {
			ctx := context.Background()
			mbc := mocks.NewBuildChromeClient(t)
			cdl := commitDataList{
				commits: []*commitData{
					{
						build: &buildMetadata{
							buildID: 1,
						},
					},
					{
						build: &buildMetadata{
							buildID: 2,
						},
					},
				},
			}
			mbc.On("GetStatus", ctx, mock.Anything).Return(bpb.Status_STATUS_UNSPECIFIED, skerr.Fmt("some error")).Once()
			c, err := cdl.pollBuild(ctx, mbc)
			So(c, ShouldBeNil)
			So(err, ShouldErrLike, "Could not get build status")
		})
	})
}

func TestScheduleRunBenchmark(t *testing.T) {
	ctx := context.Background()
	msc := swarmingMocks.NewApiClient(t)
	req := defaultRunRequest()

	Convey(`OK`, t, func() {
		c := &commitData{
			commit: &midpoint.Commit{
				GitHash: "commit hash",
			},
			build: &buildMetadata{
				buildCAS: &swarmingV1.SwarmingRpcsCASReference{
					Digest: &swarmingV1.SwarmingRpcsDigest{
						Hash:      "hash",
						SizeBytes: 123,
					},
				},
			},
		}
		cfg, err := bot_configs.GetBotConfig(req.Device, false)
		So(err, ShouldBeNil)
		c.tests = &testMetadata{
			req: c.createRunBenchmarkRequest("jobID", cfg, "target", req),
		}

		msc.On("ListTasks", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()
		msc.On("TriggerTask", ctx, mock.Anything).Return(
			&swarmingV1.SwarmingRpcsTaskRequestMetadata{
				TaskId: "new_task",
			}, nil).Times(interval)

		tasks, err := c.scheduleRunBenchmark(ctx, msc)
		So(err, ShouldBeNil)
		So(tasks[0], ShouldEqual, "new_task")
		So(len(tasks), ShouldEqual, interval)
	})

	Convey(`Error`, t, func() {
		Convey(`When no tests started`, func() {
			c := &commitData{}
			tasks, err := c.scheduleRunBenchmark(ctx, msc)
			So(err, ShouldErrLike, "Cannot schedule benchmark runs without request")
			So(tasks, ShouldBeNil)
		})
		Convey(`When no request`, func() {
			c := &commitData{
				tests: &testMetadata{},
			}
			tasks, err := c.scheduleRunBenchmark(ctx, msc)
			So(err, ShouldErrLike, "Cannot schedule benchmark runs without request")
			So(tasks, ShouldBeNil)
		})
		Convey(`When client fails to start new tasks`, func() {
			c := &commitData{
				commit: &midpoint.Commit{
					GitHash: "commit hash",
				},
				build: &buildMetadata{
					buildCAS: &swarmingV1.SwarmingRpcsCASReference{
						Digest: &swarmingV1.SwarmingRpcsDigest{
							Hash:      "hash",
							SizeBytes: 123,
						},
					},
				},
			}
			cfg, err := bot_configs.GetBotConfig(req.Device, false)
			So(err, ShouldBeNil)
			c.tests = &testMetadata{
				req: c.createRunBenchmarkRequest("jobID", cfg, "target", req),
			}

			errMsg := "some error"
			msc.On("ListTasks", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(nil, nil).Once()
			msc.On("TriggerTask", ctx, mock.Anything).Return(nil, skerr.Fmt(errMsg)).Once()

			tasks, err := c.scheduleRunBenchmark(ctx, msc)
			So(err, ShouldErrLike, errMsg)
			So(tasks, ShouldBeNil)
		})
	})
}

func TestPollTests(t *testing.T) {
	ctx := context.Background()
	msc := swarmingMocks.NewApiClient(t)
	Convey(`OK`, t, func() {
		Convey(`When no tasks to poll`, func() {
			cdl := commitDataList{
				commits: []*commitData{},
			}
			idx, c, err := cdl.pollTests(ctx, msc)
			So(err, ShouldBeNil)
			So(idx, ShouldEqual, -1)
			So(c, ShouldBeNil)

			cdl = commitDataList{
				commits: []*commitData{
					{
						tests: &testMetadata{},
					},
				},
			}
			idx, c, err = cdl.pollTests(ctx, msc)
			So(err, ShouldBeNil)
			So(idx, ShouldEqual, -1)
			So(c, ShouldBeNil)
			So(cdl.commits[0].tests.isRunning, ShouldBeFalse)
		})
		Convey(`When tasks are already finished`, func() {
			cdl := commitDataList{
				commits: []*commitData{
					{
						tests: &testMetadata{
							isRunning: false,
						},
					},
				},
			}
			idx, c, err := cdl.pollTests(ctx, msc)
			So(err, ShouldBeNil)
			So(idx, ShouldEqual, -1)
			So(c, ShouldBeNil)
			So(cdl.commits[0].tests.isRunning, ShouldBeFalse)
		})
		Convey(`When running tasks are still running`, func() {
			cdl := commitDataList{
				commits: []*commitData{
					{
						commit: &midpoint.Commit{
							GitHash: "fake-git-string",
						},
						tests: &testMetadata{
							tasks:     []string{"1", "2"},
							isRunning: true,
						},
					},
				},
			}
			msc.On("GetStates", ctx, mock.Anything).Return(
				[]string{
					"RUNNING",
					"COMPLETED",
				}, nil).Once()

			idx, c, err := cdl.pollTests(ctx, msc)
			So(err, ShouldBeNil)
			So(idx, ShouldEqual, -1)
			So(c, ShouldBeNil)
			So(cdl.commits[0].tests.isRunning, ShouldBeTrue)
		})
		Convey(`When tasks just finish`, func() {
			cdl := commitDataList{
				commits: []*commitData{
					{
						commit: &midpoint.Commit{
							GitHash: "fake-git-string",
						},
						tests: &testMetadata{
							tasks:     []string{"1", "2"},
							isRunning: true,
						},
					},
				},
			}
			msc.On("GetStates", ctx, mock.Anything).Return(
				[]string{
					"FAILURE",
					"COMPLETED",
				}, nil).Once()

			idx, c, err := cdl.pollTests(ctx, msc)
			So(err, ShouldBeNil)
			So(idx, ShouldEqual, 0)
			So(c, ShouldNotBeNil)
			So(cdl.commits[0].tests.isRunning, ShouldBeFalse)
		})
	})
	Convey(`Error when client fails`, t, func() {
		cdl := commitDataList{
			commits: []*commitData{
				{
					commit: &midpoint.Commit{
						GitHash: "fake-git-string",
					},
					tests: &testMetadata{
						tasks:     []string{"1", "2"},
						isRunning: true,
					},
				},
			},
		}
		msc.On("GetStates", ctx, mock.Anything).Return(nil, skerr.Fmt("some error")).Once()

		idx, c, err := cdl.pollTests(ctx, msc)
		So(err, ShouldErrLike, "failed to retrieve swarming tasks")
		So(idx, ShouldEqual, -1)
		So(c, ShouldBeNil)
	})
}

func TestGetTestCAS(t *testing.T) {
	ctx := context.Background()
	msc := swarmingMocks.NewApiClient(t)
	Convey(`OK`, t, func() {
		c := &commitData{
			tests: &testMetadata{
				tasks:  []string{"1", "2"},
				states: []string{"COMPLETED", "COMPLETED"},
			},
		}
		msc.On("GetTask", ctx, mock.Anything, false).Return(
			&swarmingV1.SwarmingRpcsTaskResult{
				State: "COMPLETED",
				CasOutputRoot: &swarmingV1.SwarmingRpcsCASReference{
					CasInstance: "instance",
					Digest: &swarmingV1.SwarmingRpcsDigest{
						Hash:      "hash",
						SizeBytes: 123,
					},
				},
			}, nil,
		).Times(len(c.tests.tasks))

		cas, err := c.getTestCAS(ctx, msc)
		So(err, ShouldBeNil)
		So(cas, ShouldNotBeNil)
		So(len(cas), ShouldEqual, len(c.tests.tasks))
		So(cas[0].CasInstance, ShouldEqual, "instance")
		So(cas[0].Digest.Hash, ShouldEqual, "hash")
		So(cas[0].Digest.SizeBytes, ShouldEqual, 123)
	})

	Convey(`Error`, t, func() {
		Convey(`When there are no tests`, func() {
			c := &commitData{}
			cas, err := c.getTestCAS(ctx, msc)
			So(err, ShouldErrLike, "cannot get cas output of non-existent swarming tasks")
			So(cas, ShouldBeNil)
			So(c.tests, ShouldBeNil)
		})
		Convey(`When tests and states are not equal length`, func() {
			c := &commitData{
				tests: &testMetadata{
					tasks:  []string{"1", "2"},
					states: []string{"COMPLETED"},
				},
			}
			cas, err := c.getTestCAS(ctx, msc)
			So(err, ShouldErrLike, "mismatching number of swarming states")
			So(cas, ShouldBeNil)
			So(c.tests.casOutputs, ShouldBeNil)
		})
		Convey(`When client fails`, func() {
			c := &commitData{
				tests: &testMetadata{
					tasks:  []string{"1", "2"},
					states: []string{"COMPLETED", "COMPLETED"},
				},
			}
			msc.On("GetTask", ctx, mock.Anything, false).Return(nil,
				skerr.Fmt("some error"),
			).Once()

			cas, err := c.getTestCAS(ctx, msc)
			So(err, ShouldErrLike, "error retrieving cas outputs")
			So(cas, ShouldBeNil)
		})
	})
}

// TODO(sunxiaodi@): add interface to read_values to mock
// successful scenarios
func TestGetValues(t *testing.T) {
	ctx := context.Background()
	mrc := &rbeclient.Client{} // create mock client
	Convey(`Error`, t, func() {
		Convey(`When there are no tests`, func() {
			c := &commitData{}
			values, err := c.getValues(ctx, mrc, defaultRunRequest())
			So(err, ShouldErrLike, "cannot retrieve values with no swarming tests")
			So(values, ShouldBeNil)
		})
		Convey(`When there are no cas outputs`, func() {
			c := &commitData{
				tests: &testMetadata{
					tasks:  []string{"1", "2"},
					states: []string{"COMPLETED"},
				},
			}
			values, err := c.getValues(ctx, mrc, defaultRunRequest())
			So(err, ShouldErrLike, "cannot retrieve values with no swarming test cas outputs")
			So(values, ShouldBeNil)
		})
	})
}

func TestCompareNeighbor(t *testing.T) {
	Convey(`OK`, t, func() {
		Convey(`Return nil when commit is not comparable`, func() {
			cdl := commitDataList{
				commits: []*commitData{
					{
						values: []float64{0, 0, 0, 0, 0},
					},
					{
						build:  &buildMetadata{},
						tests:  &testMetadata{},
						values: []float64{1, 1, 1, 1, 1},
					},
				},
			}
			res, err := cdl.compareNeighbor(0, 1, 0.0)
			So(err, ShouldBeNil)
			So(res, ShouldBeNil)
		})
		cdl := commitDataList{
			commits: []*commitData{
				{
					build:  &buildMetadata{},
					tests:  &testMetadata{},
					values: []float64{0, 0, 0, 0, 0},
				},
				{
					build:  &buildMetadata{},
					tests:  &testMetadata{},
					values: []float64{1, 1, 1, 1, 1},
				},
			},
		}
		Convey(`Return nil when left or right is out of bounds`, func() {
			res, err := cdl.compareNeighbor(0, 2, 0.0)
			So(err, ShouldBeNil)
			So(res, ShouldBeNil)
			res, err = cdl.compareNeighbor(-1, 1, 0.0)
			So(err, ShouldBeNil)
			So(res, ShouldBeNil)
		})
		Convey(`Return result when comparison criteria is met`, func() {
			res, err := cdl.compareNeighbor(0, 1, 0.0)
			So(err, ShouldBeNil)
			So(res.Verdict, ShouldEqual, compare.Different)
		})

	})
	Convey(`Error when left is >= right`, t, func() {
		cdl := commitDataList{
			commits: []*commitData{
				{
					build:  &buildMetadata{},
					tests:  &testMetadata{},
					values: []float64{0, 0, 0, 0, 0},
				},
				{
					build:  &buildMetadata{},
					tests:  &testMetadata{},
					values: []float64{1, 1, 1, 1, 1},
				},
			},
		}
		res, err := cdl.compareNeighbor(1, 0, 0.0)
		So(err, ShouldNotBeNil)
		So(res, ShouldBeNil)

	})
}

func TestNotComparable(t *testing.T) {
	Convey(`Return true`, t, func() {
		Convey(`When there is no build`, func() {
			c := &commitData{}
			So(c.notComparable(), ShouldBeTrue)
		})
		Convey(`When there is no test`, func() {
			c := &commitData{
				build: &buildMetadata{},
			}
			So(c.notComparable(), ShouldBeTrue)
		})
		Convey(`When test is still running`, func() {
			c := &commitData{
				build: &buildMetadata{},
				tests: &testMetadata{
					isRunning: true,
				},
			}
			So(c.notComparable(), ShouldBeTrue)
		})
		Convey(`When no values`, func() {
			c := &commitData{
				build: &buildMetadata{},
				tests: &testMetadata{
					isRunning: false,
				},
			}
			So(c.notComparable(), ShouldBeTrue)
		})
	})
	Convey(`Return false`, t, func() {
		c := &commitData{
			build: &buildMetadata{},
			tests: &testMetadata{
				isRunning: false,
			},
			values: []float64{1.0},
		}
		So(c.notComparable(), ShouldBeFalse)
	})
}

// TODO(sunxiaodi@) update unit test to include runMoreTestsIfNeeded
// and findMidpointOrCulprit to unit test the behavior better
func TestUpdateCommits(t *testing.T) {
	ctx := context.Background()
	msc := swarmingMocks.NewApiClient(t)
	c := mockhttpclient.NewURLMock().Client()
	mmh := midpoint.New(ctx, c)

	Convey(`Error`, t, func() {
		cdl := commitDataList{
			commits: []*commitData{
				{},
				{},
			},
		}
		Convey(`When index out of bounds`, func() {
			left, right := -1, 0
			res := &compare.CompareResults{}
			mid, err := cdl.updateCommitsByResult(ctx, msc, mmh, res, left, right)
			So(err, ShouldErrLike, "index out of bounds")
			So(mid, ShouldBeNil)
		})
		Convey(`When left >= right`, func() {
			left, right := 1, 0
			res := &compare.CompareResults{}
			mid, err := cdl.updateCommitsByResult(ctx, msc, mmh, res, left, right)
			So(err, ShouldErrLike, fmt.Sprintf("left %d index >= right %d", left, right))
			So(mid, ShouldBeNil)
		})
	})
}

func TestUpdateUnknown(t *testing.T) {
	ctx := context.Background()
	msc := swarmingMocks.NewApiClient(t)
	req := defaultRunRequest()
	left, right := 0, 1

	Convey(`OK`, t, func() {
		cdl := commitDataList{
			commits: []*commitData{
				{
					commit: &midpoint.Commit{
						GitHash: "left-hash",
					},
					build: &buildMetadata{
						buildCAS: &swarmingV1.SwarmingRpcsCASReference{
							Digest: &swarmingV1.SwarmingRpcsDigest{
								Hash:      "hash",
								SizeBytes: 123,
							},
						},
					},
					tests: &testMetadata{},
				},
				{
					commit: &midpoint.Commit{
						GitHash: "right-hash",
					},
					build: &buildMetadata{
						buildCAS: &swarmingV1.SwarmingRpcsCASReference{
							Digest: &swarmingV1.SwarmingRpcsDigest{
								Hash:      "hash",
								SizeBytes: 123,
							},
						},
					},
					tests: &testMetadata{},
				},
			},
		}

		lcommit := cdl.commits[left]
		rcommit := cdl.commits[right]
		cfg, err := bot_configs.GetBotConfig(req.Device, false)
		So(err, ShouldBeNil)
		cdl.commits[left].tests = &testMetadata{
			req: lcommit.createRunBenchmarkRequest("jobID", cfg, "target", req),
		}
		cdl.commits[right].tests = &testMetadata{
			req: rcommit.createRunBenchmarkRequest("jobID", cfg, "target", req),
		}

		msc.On("ListTasks", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			[]*swarmingV1.SwarmingRpcsTaskRequestMetadata{
				{
					TaskId: "old_left_task_1",
				},
				{
					TaskId: "old_left_task_2",
				},
			}, nil).Once()
		msc.On("ListTasks", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(
			[]*swarmingV1.SwarmingRpcsTaskRequestMetadata{
				{
					TaskId: "old_right_task_1",
				},
			}, nil).Once()
		msc.On("TriggerTask", ctx, mock.Anything).Return(
			&swarmingV1.SwarmingRpcsTaskRequestMetadata{
				TaskId: "new_left_task",
			}, nil).Times(interval)
		msc.On("TriggerTask", ctx, mock.Anything).Return(
			&swarmingV1.SwarmingRpcsTaskRequestMetadata{
				TaskId: "new_right_task",
			}, nil).Times(interval)

		err = cdl.runMoreTestsIfNeeded(ctx, msc, left, right)
		So(err, ShouldBeNil)
		So(len(lcommit.tests.tasks), ShouldEqual, interval+2)
		So(lcommit.tests.tasks[0], ShouldEqual, "old_left_task_1")
		So(lcommit.tests.tasks[2], ShouldEqual, "new_left_task")
		So(len(rcommit.tests.tasks), ShouldEqual, interval+1)
		So(rcommit.tests.tasks[0], ShouldEqual, "old_right_task_1")
		So(rcommit.tests.tasks[2], ShouldEqual, "new_right_task")
	})
}
