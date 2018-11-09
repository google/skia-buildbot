package td

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/pborman/uuid"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	MAX_STEP_NAME_CHARS = 100

	STEP_RESULT_SUCCESS   StepResult = "SUCCESS"
	STEP_RESULT_FAILURE   StepResult = "FAILURE"
	STEP_RESULT_EXCEPTION StepResult = "EXCEPTION"
)

// StepResult represents the result of a Step.
type StepResult string

// Start a new step, returning a context.Context associated with it.
func newStep(ctx context.Context, id string, parent *StepProperties, props *StepProperties) context.Context {
	if props == nil {
		props = &StepProperties{}
	}
	props.Id = id
	if parent != nil {
		// If empty, steps inherit their environment from their parent
		// step.
		// TODO(borenet): Should we merge environments?
		if len(props.Environ) == 0 {
			props.Environ = parent.Environ
		}

		// Steps inherit the infra status of their parent.
		// TODO(borenet): What if we want to have a parent which is an
		// infra step but a child which is not?
		if parent.IsInfra {
			props.IsInfra = true
		}

		props.Parent = parent.Id
	}
	ctx = setStep(ctx, props)
	ctx = execCtx(ctx)
	getRun(ctx).Start(props)
	return ctx
}

// Create a step.
func StartStep(ctx context.Context, props *StepProperties) context.Context {
	parent := getStep(ctx)
	return newStep(ctx, uuid.New(), parent, props)
}

// infraErrors collects all infrastructure errors.
var infraErrors = map[error]bool{}

// IsInfraError returns true if the given error is an infrastructure error.
func IsInfraError(err error) bool {
	return infraErrors[err]
}

// InfraError wraps the given error, indicating that it is an infrastructure-
// related error. If the given error is already an InfraError, returns it as-is.
func InfraError(err error) error {
	infraErrors[err] = true
	return err
}

// Mark the step as failed, with the given error. Returns the passed-in error
// for convenience, so that the caller can do things like:
//
//	if err := doSomething(); err != nil {
//		return FailStep(ctx, err)
//	}
//
func FailStep(ctx context.Context, err error) error {
	props := getStep(ctx)
	if props.IsInfra {
		err = InfraError(err)
	}
	getRun(ctx).Failed(props.Id, err)
	return err
}

// Mark the Step as finished. This is intended to be used in a defer, eg.
//
//	ctx = td.StartStep(ctx)
//	defer td.EndStep(ctx)
//
// If a panic is recovered in EndStep, the step is marked as failed and the
// panic is re-raised.
func EndStep(ctx context.Context) {
	finishStep(ctx, recover())
}

// finishStep is a helper function for EndStep which is also used by
// RunFinished to set the result of the root step.
func finishStep(ctx context.Context, recovered interface{}) {
	props := getStep(ctx)
	e := getRun(ctx)
	if recovered != nil {
		// If the panic is an error, use the original error, otherwise
		// create an error.
		err, ok := recovered.(error)
		if !ok {
			err = InfraError(fmt.Errorf("Caught panic: %v", recovered))
		}
		e.Failed(props.Id, err)
		defer panic(recovered)
	}
	e.Finish(props.Id)
}

// Attach the given StepData to this Step.
func StepData(ctx context.Context, typ DataType, d interface{}) {
	props := getStep(ctx)
	getRun(ctx).AddStepData(props.Id, typ, d)
}

// Do is a convenience function which runs the given function as a Step. It
// handles creation of the sub-step and calling EndStep() for you.
func Do(ctx context.Context, props *StepProperties, fn func(context.Context) error) error {
	ctx = StartStep(ctx, props)
	defer EndStep(ctx)
	if err := fn(ctx); err != nil {
		return FailStep(ctx, err)
	}
	return nil
}

// Fatal is a substitute for sklog.Fatal which logs an error and panics.
// sklog.Fatal does not panic but calls os.Exit, which prevents the Task Driver
// from properly reporting errors.
func Fatal(ctx context.Context, err error) {
	sklog.Error(err)
	if getStep(ctx).IsInfra {
		err = InfraError(err)
	}
	panic(err)
}

// Fatalf is a substitute for sklog.Fatalf which logs an error and panics.
// sklog.Fatalf does not panic but calls os.Exit, which prevents the Task Driver
// from properly reporting errors.
func Fatalf(ctx context.Context, format string, a ...interface{}) {
	Fatal(ctx, fmt.Errorf(format, a...))
}

// LogData is extra Step data generated for log streams.
type LogData struct {
	Name     string `json:"name"`
	Id       string `json:"id"`
	Severity string `json:"severity"`
	Log      string `json:"log,omitempty"`
}

// Create an io.Writer that will act as a log stream for this Step. Callers
// probably want to use a higher-level method instead.
func NewLogStream(ctx context.Context, name, severity string) io.Writer {
	props := getStep(ctx)
	return getRun(ctx).LogStream(props.Id, name, severity)
}

// ExecData is extra Step data generated when executing commands through the
// exec package.
type ExecData struct {
	Cmd []string `json:"command"`
	Env []string `json:"env,omitempty"`
}

// Return a context.Context associated with this Step. Any calls to exec which
// use this Context will be attached to the Step.
func execCtx(ctx context.Context) context.Context {
	return exec.NewContext(ctx, func(cmd *exec.Command) error {
		name := strings.Join(append([]string{cmd.Name}, cmd.Args...), " ")
		return Do(ctx, Props(name), func(ctx context.Context) error {
			props := getStep(ctx)
			// Inherit env from the step unless it's explicitly provided.
			// TODO(borenet): Should we merge instead?
			if len(cmd.Env) == 0 {
				cmd.Env = props.Environ
			}

			// Set up stdout and stderr streams.
			stdout := NewLogStream(ctx, "stdout", sklog.INFO)
			if cmd.Stdout != nil {
				stdout = util.MultiWriter([]io.Writer{cmd.Stdout, stdout})
			}
			cmd.Stdout = stdout
			stderr := NewLogStream(ctx, "stderr", sklog.ERROR)
			if cmd.Stderr != nil {
				stderr = util.MultiWriter([]io.Writer{cmd.Stderr, stderr})
			}
			cmd.Stderr = stderr

			// Collect step metadata about the command.
			d := &ExecData{
				Cmd: append([]string{cmd.Name}, cmd.Args...),
				Env: cmd.Env,
			}
			StepData(ctx, DATA_TYPE_COMMAND, d)

			// Run the command.
			return exec.DefaultRun(cmd)
		})
	})
}

// httpTransport is an http.RoundTripper which wraps another http.RoundTripper
// to record data about the requests it sends.
type httpTransport struct {
	ctx context.Context
	rt  http.RoundTripper
}

// HttpRequestData is Step data describing an http.Request. Notably, it does not
// include the request body or headers, to avoid leaking auth tokens or other
// sensitive information.
type HttpRequestData struct {
	Method string   `json:"method,omitempty"`
	URL    *url.URL `json:"url,omitempty"`
}

// HttpResponseData is Step data describing an http.Response. Notably, it does
// not include the response body, to avoid leaking sensitive information.
type HttpResponseData struct {
	StatusCode int `json:"status,omitempty"`
}

// See documentation for http.RoundTripper.
func (t *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	return resp, Do(t.ctx, Props(req.URL.String()), func(ctx context.Context) error {
		StepData(ctx, DATA_TYPE_HTTP_REQUEST, &HttpRequestData{
			Method: req.Method,
			URL:    req.URL,
		})
		var err error
		resp, err = t.rt.RoundTrip(req)
		if resp != nil {
			StepData(ctx, DATA_TYPE_HTTP_RESPONSE, &HttpResponseData{
				StatusCode: resp.StatusCode,
			})
		}
		return err
	})
}

// Return an http.Client which wraps the given http.Client to record data about
// the requests it sends.
func HttpClient(ctx context.Context, c *http.Client) *http.Client {
	if c == nil {
		c = http.DefaultClient // TODO(borenet): Use backoff client?
	}
	if c.Transport == nil {
		c.Transport = http.DefaultTransport
	}
	c.Transport = &httpTransport{
		ctx: ctx,
		rt:  c.Transport,
	}
	return c
}
