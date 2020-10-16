package td

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"
	multierror "github.com/hashicorp/go-multierror"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	fsnotify "gopkg.in/fsnotify.v1"
)

const (
	MAX_STEP_NAME_CHARS = 100

	STEP_RESULT_SUCCESS   StepResult = "SUCCESS"
	STEP_RESULT_FAILURE   StepResult = "FAILURE"
	STEP_RESULT_EXCEPTION StepResult = "EXCEPTION"

	// PATH_PLACEHOLDER is a placeholder for any existing value of PATH,
	// used when merging environments to avoid overriding the PATH
	// altogether.
	PATH_PLACEHOLDER = "%(PATH)s"
)

// Merge the second env into the base, returning a new env with the original
// unchanged. Variables in the second env override those in the base, except
// for PATH, which is merged. If PATH defined by the second env contains
// %(PATH)s, then the result is the PATH from the second env with PATH from
// the first env inserted in place of %(PATH)s. Otherwise, the PATH from the
// second env overrides PATH from the first. Note that setting PATH to the empty
// string in the second env will cause PATH to be empty in the result!
func MergeEnv(base, other []string) []string {
	m := make(map[string]string, len(base))
	for _, kv := range base {
		split := strings.SplitN(kv, "=", 2)
		if len(split) != 2 {
			sklog.Errorf("Invalid env var: %s", kv)
			continue
		}
		m[split[0]] = split[1]
	}
	for _, kv := range other {
		split := strings.SplitN(kv, "=", 2)
		if len(split) != 2 {
			sklog.Errorf("Invalid env var: %s", kv)
			continue
		}
		k, v := split[0], split[1]
		if existing, ok := m[k]; ok && k == PATH_VAR {
			if strings.Contains(v, PATH_PLACEHOLDER) {
				v = strings.Replace(v, PATH_PLACEHOLDER, existing, -1)
			}
		}
		m[k] = v
	}
	rv := make([]string, 0, len(m))
	for k, v := range m {
		rv = append(rv, fmt.Sprintf("%s=%s", k, v))
	}
	sort.Strings(rv)
	return rv
}

// StepResult represents the result of a Step.
type StepResult string

// Start a new step, returning a context.Context associated with it.
func newStep(ctx context.Context, id string, parent *StepProperties, props *StepProperties) context.Context {
	if props == nil {
		props = &StepProperties{}
	}
	props.Id = id

	if parent != nil {
		// Steps inherit the infra status of their parent.
		// TODO(borenet): What if we want to have a parent which is an
		// infra step but a child which is not?
		if parent.IsInfra {
			props.IsInfra = true
		}

		props.Parent = parent.Id
	}
	ctx = withChildCtx(ctx, &Context{
		step: props,
	})
	getCtx(ctx).run.Start(props)
	return ctx
}

// Create a step.
func StartStep(ctx context.Context, props *StepProperties) context.Context {
	parent := getCtx(ctx).step
	return newStep(ctx, uuid.New().String(), parent, props)
}

// infraErrors collects all infrastructure errors.
var infraErrors = map[error]bool{}
var infraErrorsMtx sync.Mutex

// IsInfraError returns true if the given error is an infrastructure error.
func IsInfraError(err error) bool {
	infraErrorsMtx.Lock()
	defer infraErrorsMtx.Unlock()
	return infraErrors[err]
}

// InfraError wraps the given error, indicating that it is an infrastructure-
// related error. If the given error is already an InfraError, returns it as-is.
func InfraError(err error) error {
	infraErrorsMtx.Lock()
	defer infraErrorsMtx.Unlock()
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
	props := getCtx(ctx).step
	if props.IsInfra {
		err = InfraError(err)
	}
	getCtx(ctx).run.Failed(props.Id, err)
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
	props := getCtx(ctx).step
	e := getCtx(ctx).run
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
	props := getCtx(ctx).step
	getCtx(ctx).run.AddStepData(props.Id, typ, d)
}

// StepText displays the provided text with the label in the Step's UI. The
// text will be escaped and URLs in it will be linkified.
func StepText(ctx context.Context, label, value string) {
	d := &TextData{
		Label: label,
		Value: value,
	}
	StepData(ctx, DATA_TYPE_TEXT, d)
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
	if getCtx(ctx).step.IsInfra {
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

// WithRetries runs the given function until it succeeds or the given number of
// attempts is exhausted, returning the last error or nil if the function
// completed successfully.
func WithRetries(ctx context.Context, attempts int, fn func(ctx context.Context) error) error {
	var err error
	for i := 0; i < attempts; i++ {
		err = fn(ctx)
		if err == nil {
			return nil
		}
	}
	return err
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
func NewLogStream(ctx context.Context, name string, severity Severity) io.Writer {
	props := getCtx(ctx).step
	return getCtx(ctx).run.LogStream(props.Id, name, severity)
}

// FileStream is a struct used for streaming logs from a file, eg. when a test
// program writes verbose logs to a file. Intended to be used like this:
//
//	fs := s.NewFileStream("verbose")
//	defer util.Close(fs)
//	_, err := s.RunCwd(".", myTestProg, "--verbose", fs.FilePath())
//
type FileStream struct {
	cancel  context.CancelFunc
	ctx     context.Context
	doneCh  <-chan struct{}
	err     *multierror.Error
	file    *os.File
	name    string
	w       io.Writer
	watcher *fsnotify.Watcher
}

// Create a log stream which uses an intermediate file, eg. for writing from a
// test program.
func NewFileStream(ctx context.Context, name string, severity Severity) (*FileStream, error) {
	w := NewLogStream(ctx, name, severity)
	f, err := ioutil.TempFile("", "log")
	if err != nil {
		return nil, fmt.Errorf("Failed to create file-based log stream; failed to create log file: %s", err)
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, fmt.Errorf("Failed to create file-based log stream; failed to create fsnotify.Watcher: %s", err)
	}
	if err := watcher.Add(f.Name()); err != nil {
		return nil, fmt.Errorf("Failed to create file-based log stream; failed to add a watcher for the log file: %s", err)
	}
	ctx, cancel := context.WithCancel(ctx)
	doneCh := make(chan struct{})
	rv := &FileStream{
		cancel:  cancel,
		ctx:     ctx,
		doneCh:  doneCh,
		file:    f,
		name:    name,
		w:       w,
		watcher: watcher,
	}

	// Start collecting logs from the file.
	go rv.follow(doneCh)
	return rv, nil
}

// Read from the file incrementally as it is written, writing its contents to
// the step's log emitter.
func (fs *FileStream) follow(doneCh chan<- struct{}) {
	reportErr := func(format string, args ...interface{}) {
		err := fmt.Errorf(format, args...)
		getCtx(fs.ctx).run.Failed(getCtx(fs.ctx).step.Id, InfraError(err))
		fs.err = multierror.Append(fs.err, err)
	}

	defer func() {
		// Cleanup.
		if err := fs.file.Close(); err != nil {
			reportErr("Failed to close logstream file: %s", err)
		}
		if err := fs.watcher.Close(); err != nil {
			reportErr("Failed to close logstream file watcher: %s", err)
		}
		if err := os.Remove(fs.file.Name()); err != nil {
			reportErr("Failed to delete logstream file: %s", err)
		}
		doneCh <- struct{}{}
	}()

	buf := make([]byte, 128)
	for {
		select {
		case <-fs.ctx.Done():
			// fs.Close() was called; return.
			return
		case <-fs.watcher.Events:
			// The file was modified in some way; continue reading,
			// assuming that it was appended.
			for {
				nRead, err := fs.file.Read(buf)
				// Technically, an io.Reader is allowed to return
				// non-zero number of bytes read AND io.EOF on the
				// same call to Read(). Don't handle EOF until we've
				// written all of the data we read.
				if err != nil && err != io.EOF {
					reportErr("Failed to read from logstream file: %s", err)
					return
				}
				if nRead > 0 {
					nWrote, err := fs.w.Write(buf[:nRead])
					if err != nil {
						reportErr("Failed to write to log stream: %s", err)
						return
					}
					if nWrote != nRead {
						reportErr("Read %d bytes but wrote %d!", nRead, nWrote)
						return
					}
				}
				if err == io.EOF {
					break
				}
			}
		case err := <-fs.watcher.Errors:
			reportErr("fsnotify watcher error: %s", err)
			return
		}
	}
}

// Close the FileStream, cleaning up its resources and deleting the log file.
func (fs *FileStream) Close() error {
	fs.cancel()
	_ = <-fs.doneCh
	return fs.err.ErrorOrNil()
}

// Return the path to the logfile used by this FileStream.
func (fs *FileStream) FilePath() string {
	return fs.file.Name()
}

// TextData is extra Step data for displaying a text in a step. The provided
// text will be escaped and URLs in it will be linkified.
type TextData struct {
	Label string `json:"label"`
	Value string `json:"value"`
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
	return exec.NewContext(ctx, func(ctx context.Context, cmd *exec.Command) error {
		name := strings.Join(append([]string{cmd.Name}, cmd.Args...), " ")

		// Merge the command's env into that of its parent.
		cmd.Env = MergeEnv(getCtx(ctx).env, cmd.Env)

		return Do(ctx, Props(name).Env(cmd.Env), func(ctx context.Context) error {
			// Set up stdout and stderr streams.
			stdout := NewLogStream(ctx, "stdout", Info)
			if cmd.Stdout != nil {
				stdout = util.MultiWriter([]io.Writer{cmd.Stdout, stdout})
			}
			cmd.Stdout = stdout
			stderr := NewLogStream(ctx, "stderr", Error)
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
			return getCtx(ctx).execRun(ctx, cmd)
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
	ctx := req.Context()
	if ctx.Value(contextKey) == nil {
		// Fallback in case the caller did not set a context on the request.
		ctx = t.ctx
	}
	return resp, Do(ctx, Props(fmt.Sprintf("%s %s", req.Method, req.URL.String())), func(ctx context.Context) error {
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
		c = httputils.DefaultClientConfig().Client()
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

// MustGetAbsolutePathOfFlag returns the absolute path to the specified path or exits with an
// error indicating that the given flag must be specified.
func MustGetAbsolutePathOfFlag(ctx context.Context, nonEmptyPath, flag string) string {
	if nonEmptyPath == "" {
		Fatalf(ctx, "--%s must be specified", flag)
	}
	absPath, err := filepath.Abs(nonEmptyPath)
	if err != nil {
		Fatal(ctx, skerr.Wrap(err))
	}
	return absPath
}
