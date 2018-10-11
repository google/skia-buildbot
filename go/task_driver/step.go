package task_driver

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

	STEP_RESULT_SUCCESS   = "SUCCESS"
	STEP_RESULT_FAILED    = "FAILED"
	STEP_RESULT_EXCEPTION = "EXCEPTION"
)

// StepProperties are basic properties of a Step.
type StepProperties struct {
	Id      string   `json:"id"`
	Name    string   `json:"name"`
	IsInfra bool     `json:"isInfra"`
	Env     []string `json:"environment,omitempty"`
	Parent  string   `json:"parent,omitempty"`
}

// StepResult contains the results of a Step.
type StepResult struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// Return a Step instance.
func newStep(ctx context.Context, id string, parent *StepProperties, opts []StepOption) context.Context {
	s := &StepProperties{
		Id: id,
	}
	if parent != nil {
		// Steps inherit their environment from their parent step.
		s.Env = parent.Env
		s.Parent = parent.Id
	}
	for _, opt := range opts {
		opt.Apply(s)
	}
	ctx = setStep(ctx, s)
	ctx = execCtx(ctx)
	getRun(ctx).emitter.Start(s)
	return ctx
}

// StepOption is an interface used to apply optional properties to a step.
type StepOption interface {
	Apply(s *StepProperties)
}

// Opts collects StepOptions into a slice.
func Opts(opts ...StepOption) []StepOption {
	return opts
}

// nameOption assigns a name to the step.
type nameOption string

// See documentation for StepOption interface.
func (o nameOption) Apply(s *StepProperties) {
	s.Name = string(o)
}

// Name returns a StepOption which applies the given name to the step.
func Name(name string) StepOption {
	if len(name) > MAX_STEP_NAME_CHARS {
		name = name[:MAX_STEP_NAME_CHARS]
	}
	return nameOption(name)
}

// infraOption marks the step as infra-specific.
type infraOption struct{}

// See documentation for StepOption interface.
func (o *infraOption) Apply(s *StepProperties) {
	s.IsInfra = true
}

// Infra returns a StepOption which marks the step as infra-specific.
func Infra() StepOption {
	return &infraOption{}
}

// envOption applies the given environment variables to all commands run within
// this Step.
type envOption []string

// See documentation for StepOption interface.
func (o envOption) Apply(s *StepProperties) {
	// TODO(borenet): Should we merge environments?
	s.Env = o
}

// Env returns a StepOption which applies the given environment variables to
// all commands run within this Step. Note that this does NOT apply the
// variables to the environment of this process, just of subprocesses spawned
// using the context.
func Env(env []string) StepOption {
	return envOption(env)
}

// Create a Step.
func StartStep(ctx context.Context, opts ...StepOption) context.Context {
	s := getStep(ctx)
	return newStep(ctx, uuid.New(), s, opts)
}

// Mark the Step as finished. If the step has already been finished, eg. via
// Fail(), no action is taken. This is intended to be used in a defer, eg.
//
//	s := r.Step().Start()
//	defer s.StepFinished(&err)
//
// After StepFinished() is called, no more work can be associated with this Step.
func FinishStep(ctx context.Context, err *error) {
	finishStep(ctx, err, recover())
}

// finishStep is a helper function for StepFinished which is also used by
// RunFinished to set the result of the root step.
func finishStep(ctx context.Context, err *error, recovered interface{}) {
	props := getStep(ctx)
	e := getRun(ctx).emitter
	if recovered != nil {
		e.Finish(props.Id, &StepResult{
			Result: STEP_RESULT_EXCEPTION,
			Error:  fmt.Sprintf("Caught panic: %s", recovered),
		})
		panic(recovered)
	}
	if err != nil && *err != nil {
		e.Finish(props.Id, &StepResult{
			Result: STEP_RESULT_FAILED,
			Error:  (*err).Error(),
		})
	} else {
		e.Finish(props.Id, &StepResult{
			Result: STEP_RESULT_SUCCESS,
		})
	}
}

// Attach the given StepData to this Step.
func StepData(ctx context.Context, d interface{}) {
	props := getStep(ctx)
	getRun(ctx).emitter.AddStepData(props.Id, d)
}

// Do is a convenience function which runs the given function as a Step. It
// handles creation of the sub-step and calling StepFinished() for you.
func Do(ctx context.Context, opts []StepOption, fn func(context.Context) error) (err error) {
	ctx = StartStep(ctx, opts...)
	defer FinishStep(ctx, &err)
	return fn(ctx)
}

// execData is extra Step data generated when executing commands through the
// exec package.
type execData struct {
	Cmd []string `json:"command"`
	Env []string `json:"env,omitempty"`
}

// logData is extra Step data generated for log streams.
type logData struct {
	Name     string `json:"name"`
	Id       string `json:"id"`
	Severity string `json:"severity"`
	Log      string `json:"log,omitempty"`
}

// Create an io.Writer that will act as a log stream for this Step. Callers
// probably want to use a higher-level method instead.
func NewLogStream(ctx context.Context, name, severity string) io.Writer {
	props := getStep(ctx)
	id := uuid.New() // TODO(borenet): Come up with a better ID.
	stream := getRun(ctx).emitter.LogStream(props.Id, id, severity)
	// Emit step data for the log stream.
	StepData(ctx, &logData{
		Name:     name,
		Id:       id,
		Severity: severity,
	})
	return stream
}

// Return a context.Context associated with this Step. Any calls to exec which
// use this Context will be attached to the Step.
func execCtx(ctx context.Context) context.Context {
	return exec.NewContext(ctx, func(cmd *exec.Command) error {
		name := strings.Join(append([]string{cmd.Name}, cmd.Args...), " ")
		return Do(ctx, Opts(Name(name)), func(ctx context.Context) error {
			props := getStep(ctx)
			// Inherit env from the step unless it's explicitly provided.
			// TODO(borenet): Should we merge instead?
			if cmd.Env == nil {
				cmd.Env = props.Env
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
			d := &execData{
				Cmd: append([]string{cmd.Name}, cmd.Args...),
				Env: cmd.Env,
			}
			StepData(ctx, d)

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

// httpRequestData is Step data describing an http.Request. Notably, it does not
// include the request body or headers, to avoid leaking auth tokens or other
// sensitive information.
type httpRequestData struct {
	Method string   `json:"method,omitempty"`
	URL    *url.URL `json:"url,omitempty"`
}

// httpResponseData is Step data describing an http.Response. Notably, it does
// not include the response body, to avoid leaking sensitive information.
type httpResponseData struct {
	StatusCode int `json:"status,omitempty"`
}

// See documentation for http.RoundTripper.
func (t *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	return resp, Do(t.ctx, Opts(Name(req.URL.String())), func(ctx context.Context) error {
		StepData(ctx, &httpRequestData{
			Method: req.Method,
			URL:    req.URL,
		})
		var err error
		resp, err = t.rt.RoundTrip(req)
		if resp != nil {
			StepData(ctx, &httpResponseData{
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
