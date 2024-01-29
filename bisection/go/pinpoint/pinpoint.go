package pinpoint

import (
	"context"
	"net/http"

	"github.com/google/uuid"
	"go.skia.org/infra/bisection/go/bot_configs"
	"go.skia.org/infra/bisection/go/read_values"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"golang.org/x/oauth2/google"
)

const (
	MissingRequiredParamTemplate = "Missing required param %s"
	chromiumSrcGit               = "https://chromium.googlesource.com/chromium/src.git"
)

// PinpointHandler is an interface to run Pinpoint jobs
type PinpointHandler interface {
	// ScheduleRun schedules a Pinpoint job and will poll the job until it is completed.
	// Only bisections are implemented at the moment, so the job will print out a list
	// of culprit commits when the job is finished.
	// jobID is an optional argument for local testing. Setting the same
	// jobID can reuse swarming results which can be helpful to triage
	// the workflow and not wait on tasks to finish.
	ScheduleRun(ctx context.Context, req PinpointRunRequest, jobID string) (*PinpointRunResponse, error)
}

// PinpointRunRequest is the request arguments to run a Pinpoint job.
type PinpointRunRequest struct {
	// Device is the device to test Chrome on i.e. linux-perf
	Device string
	// Benchmark is the benchmark to test
	Benchmark string
	// Story is the benchmark's story to test
	Story string
	// Chart is the story's subtest to measure. Only used in bisections.
	Chart string
	// Magnitude is the expected absolute difference of a potential regression.
	// Only used in bisections. Default is 1.0.
	Magnitude float64
	// AggregationMethod is the method to aggregate the measurements after a single
	// benchmark runs. Some benchmarks will output multiple values in one
	// run. Aggregation is needed to be consistent with perf measurements.
	// Only used in bisection.
	AggregationMethod read_values.AggDataMethodEnum
	// StartCommit is the base or start commit hash to run
	StartCommit string
	// EndCommit is the experimental or end commit hash to run
	EndCommit string
}

type PinpointRunResponse struct {
	// JobID is the unique job ID.
	JobID string
	// Culprits is a list of culprits found in a bisection run.
	Culprits []string
}

// pinpointJobImpl implements the PinpointJob interface.
type pinpointHandlerImpl struct {
	client *http.Client
}

func New(ctx context.Context) (*pinpointHandlerImpl, error) {
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "Problem setting up default token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()

	return &pinpointHandlerImpl{
		client: c,
	}, nil
}

// Run implements the pinpointJobImpl interface
func (pp *pinpointHandlerImpl) ScheduleRun(ctx context.Context, req PinpointRunRequest, jobID string) (
	*PinpointRunResponse, error) {
	if jobID == "" {
		jobID = uuid.New().String()
	}
	err := pp.validateRunRequest(req)
	if err != nil {
		return nil, skerr.Wrapf(err, "Could not validate request inputs")
	}

	resp := &PinpointRunResponse{
		JobID:    jobID,
		Culprits: []string{},
	}
	return resp, nil
}

// validateInputs validates the request args and returns an error if there request is invalid
func (job *pinpointHandlerImpl) validateRunRequest(req PinpointRunRequest) error {
	if req.StartCommit == "" {
		return skerr.Fmt(MissingRequiredParamTemplate, "start commit")
	}
	if req.EndCommit == "" {
		return skerr.Fmt(MissingRequiredParamTemplate, "end commit")
	}
	_, err := bot_configs.GetBotConfig(req.Device, false)
	if err != nil {
		return skerr.Wrapf(err, "Device %s not allowed in bot configurations", req.Device)
	}
	if req.Benchmark == "" {
		return skerr.Fmt(MissingRequiredParamTemplate, "benchmark")
	}
	if req.Story == "" {
		return skerr.Fmt(MissingRequiredParamTemplate, "story")
	}
	if req.Chart == "" {
		return skerr.Fmt(MissingRequiredParamTemplate, "chart")
	}
	return nil
}
