package test_automation

import (
	"context"
	"net/http"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	MSG_TYPE_STEP_STARTED  = "STEP_STARTED"
	MSG_TYPE_STEP_FINISHED = "STEP_FINISHED"
	MSG_TYPE_STEP_DATA     = "STEP_DATA"
)

// Message is a struct used to send Step metadata to Receivers.
type Message struct {
	// Type indicates the type of message, which dictates which fields must
	// be filled.
	Type string `json:"type"`

	// StepId indicates the ID for the step. This is required for every
	// Message.
	StepId string `json:"stepId"`

	// Step is the metadata about the step at creation time. Required for
	// MSG_TYPE_STEP_STARTED.
	Step *Step `json:"step,omitempty"`

	// Result is the result of the step. Required for
	// MSG_TYPE_STEP_FINISHED.
	Result *StepResult `json:"result,omitempty"`

	// Data is arbitrary additional data about the step. Required for
	// MSG_TYPE_STEP_DATA.
	Data interface{} `json:"data,omitempty"`
}

// stepEmitter is used to send metadata about Steps to various Receivers.
type stepEmitter struct {
	currentStepId string
	receivers     map[string]Receiver
}

// Send the given message to all receivers. Does not return an error, even if
// sending fails.
func (sc *stepEmitter) send(msg *Message) {
	g := util.NewNamedErrGroup()
	for k, v := range sc.receivers {
		receiver := v
		g.Go(k, func() error {
			err := receiver.HandleMessage(msg)
			return err
		})
	}
	if err := g.Wait(); err != nil {
		// Just log the error but don't return it.
		// TODO(borenet): How do we handle this?
		sklog.Error(err)
	}
}

// Send a Message indicating that a new Step has started.
func (sc *stepEmitter) Start(s *Step) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_STARTED,
		StepId: s.Id,
		Step:   s,
	}
	sc.currentStepId = s.Id
	sc.send(msg)
}

// Send a Message with additional data for the current Step.
func (sc *stepEmitter) AddStepData(d interface{}) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_DATA,
		StepId: sc.currentStepId,
		Data:   d,
	}
	sc.send(msg)
}

// Send a Message indicating that the current Step has finished with the given
// StepResult.
func (sc *stepEmitter) Finish(result *StepResult) {
	msg := &Message{
		Type:   MSG_TYPE_STEP_FINISHED,
		StepId: sc.currentStepId,
		Result: result,
	}
	sc.currentStepId = ""
	sc.send(msg)
}

// execData is extra Step data generated when executing commands through the
// exec package.
type execData struct {
	Cmd []string `json:"command"`
}

// Return a context.Context which uses the given function to run exec.Commands.
func (sc *stepEmitter) execCtx(ctx context.Context, runFn func(*exec.Command) error) context.Context {
	return exec.NewContext(ctx, func(cmd *exec.Command) error {
		sc.AddStepData(&execData{
			Cmd: append([]string{cmd.Name}, cmd.Args...),
		})
		return runFn(cmd)
		// TODO(borenet): stdout/stderr should probably get added as
		// step data. Better yet, we should stream the logs as they're
		// written. Not sure if we should do that by sending lots of
		// messages, or if we should have a separate log receiver.
	})
}

// Return a context.Context which wraps exec.DefaultRun to record data about the
// Commands it runs.
func (sc *stepEmitter) ExecCtx(ctx context.Context) context.Context {
	return sc.execCtx(ctx, func(cmd *exec.Command) error {
		return exec.DefaultRun(cmd)
	})
}

// Return a context.Context which stubs out the exec run function for testing.
func (sc *stepEmitter) ExecCtxTesting(ctx context.Context) context.Context {
	return sc.execCtx(ctx, func(cmd *exec.Command) error {
		// TODO(borenet): Fake stdout/stderr/error/etc.
		return nil
	})
}

// httpData is extra Step data generated when sending HTTP requests.
type httpData http.Request

// httpTransport is an http.RoundTripper which wraps another http.RoundTripper
// to record data about the requests it sends.
type httpTransport struct {
	sc *stepEmitter
	rt http.RoundTripper
}

// See documentation for http.RoundTripper.
func (t *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	t.sc.AddStepData(req)
	// TODO(borenet): Record response?
	return t.rt.RoundTrip(req)
}

// httpTestingTransport is an http.RoundTripper which stubs out RoundTrip for
// testing.
type httpTestingTransport struct{}

// See documentation for http.RoundTripper.
func (t *httpTestingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	// TODO(borenet): Fake response.
	return nil, nil
}

// Return an http.Client which wraps the given http.Client to record data about
// the requests it sends.
func (sc *stepEmitter) HttpClient(c *http.Client) *http.Client {
	c.Transport = &httpTransport{
		sc: sc,
		rt: c.Transport,
	}
	return c
}

// Return an http.Client which mocks requests for testing.
func (sc *stepEmitter) HttpClientTesting() *http.Client {
	return sc.HttpClient(&http.Client{
		Transport: &httpTestingTransport{},
	})
}
