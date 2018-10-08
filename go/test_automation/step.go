package test_automation

import (
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/pborman/uuid"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	fsnotify "gopkg.in/fsnotify.v1"
)

const (
	MAX_STEP_NAME_CHARS = 100

	STEP_RESULT_SUCCESS     = "SUCCESS"
	STEP_RESULT_FAILED      = "FAILED"
	STEP_RESULT_EXCEPTION   = "EXCEPTION"
	STEP_RESULT_NOT_STARTED = "NOT_STARTED"
)

// StepProperties are basic properties of a Step.
type StepProperties struct {
	Id      string   `json:"id"`
	Name    string   `json:"name"`
	IsInfra bool     `json:"isInfra"`
	Env     []string `json:"environment,omitempty"`
	Parent  string   `json:"parent,omitempty"`
}

// Step represents a single action taken inside of a test automation run.
type Step struct {
	*StepProperties
	fileStreams []*FileStream
	result      *StepResult
	run         *run
}

// StepResult contains the results of a Step.
type StepResult struct {
	Result string `json:"result"`
	Error  string `json:"error,omitempty"`
}

// Return a Step instance.
func newStep(id string, r *run, parent *Step) *Step {
	s := &Step{
		StepProperties: &StepProperties{
			Id: id,
		},
		result: &StepResult{
			Result: STEP_RESULT_NOT_STARTED,
			Error:  "Step not yet started.",
		},
		run: r,
	}
	if parent != nil {
		s.StepProperties.Env = parent.StepProperties.Env
		s.Parent = parent.Id
	}
	return s
}

// Create a new Step.
func (s *Step) Step() *Step {
	// TODO(borenet): Come up with a more systematic ID.
	return newStep(uuid.New(), s.run, s)
}

// Apply the given name to the Step.
func (s *Step) Name(name string) *Step {
	if s.IsRunning() || s.IsDone() {
		panic("Cannot modify a Step once it is running.")
	}
	if len(name) > MAX_STEP_NAME_CHARS {
		name = name[:MAX_STEP_NAME_CHARS]
	}
	s.StepProperties.Name = name
	return s
}

// Mark the Step as infra-specific.
func (s *Step) Infra() *Step {
	if s.IsRunning() || s.IsDone() {
		panic("Cannot modify a Step once it is running.")
	}
	s.IsInfra = true
	return s
}

// Apply the given environment variables to all commands run within this Step.
// Note that this does NOT apply the variables to the environment of this
// process, just of subprocesses spawned using s.Ctx().
func (s *Step) Env(env []string) *Step {
	if s.IsRunning() || s.IsDone() {
		panic("Cannot modify a Step once it is running.")
	}
	s.StepProperties.Env = env // TODO(borenet): Merge environments?
	return s
}

// Start the Step.
func (s *Step) Start() *Step {
	if s.IsRunning() {
		panic("Start() called on step which is already running.")
	}
	s.run.emitter.Start(s)
	s.result = nil
	return s
}

// Return true iff the Step has been started and has not yet finished.
func (s *Step) IsRunning() bool {
	return s.result == nil
}

// Return true iff the step has already finished.
func (s *Step) IsDone() bool {
	return s.result != nil && s.result.Result != STEP_RESULT_NOT_STARTED
}

// Mark the Step as finished with the given StepResult. After finish() is
// called, no more work can be associated with this Step.
func (s *Step) finish(res *StepResult) {
	if !s.IsRunning() {
		panic("finish() called on Step which is not running")
	}
	s.result = res
	s.run.emitter.Finish(s.Id, s.result)
}

// Mark the Step as finished. If the step has already been finished, eg. via
// Fail(), no action is taken. This is intended to be used in a defer, eg.
//
//	s := r.Step().Start()
//	defer s.Done(&err)
//
// After Done() is called, no more work can be associated with this Step.
func (s *Step) Done(err *error) {
	defer func() {
		if s.Id == STEP_ID_ROOT {
			s.run.Done()
		}
	}()
	if r := recover(); r != nil {
		s.finish(&StepResult{
			Result: STEP_RESULT_EXCEPTION,
			Error:  fmt.Sprintf("Caught panic: %s", r),
		})
		panic(r)
	} else if s.IsRunning() {
		if err != nil && *err != nil {
			s.finish(&StepResult{
				Result: STEP_RESULT_FAILED,
				Error:  (*err).Error(),
			})
		} else {
			s.finish(&StepResult{
				Result: STEP_RESULT_SUCCESS,
			})
		}
	} else {
		panic("Done() called on Step which is not running")
	}
}

// Attach the given Data to this Step. The Step must be running.
func (s *Step) Data(d interface{}) *Step {
	if !s.IsRunning() {
		panic("Data() called on Step which is not running")
	}
	s.run.emitter.AddStepData(s.Id, d)
	return s
}

// Do is a convenience function which runs the given function as a Step. It
// handles calls to Start() and Done() as appropriate.
func (s *Step) Do(fn func(*Step) error) (err error) {
	s.Start()
	defer s.Done(&err)
	return fn(s)
}

// execData is extra Step data generated when executing commands through the
// exec package.
type execData struct {
	Cmd  []string          `json:"command"`
	Env  []string          `json:"env,omitempty"`
	Logs map[string]string `json:"logs,omitempty"`
}

// Create an io.Writer that will act as a log stream for this Step. Callers
// probably want to use a higher-level method instead. Returns the Writer and
// the ID of the log stream.
func (s *Step) NewLogStream() (io.Writer, string) {
	id := uuid.New() // TODO(borenet): Come up with a better ID.
	stream := s.run.emitter.LogStream(s.Id, id)
	return stream, id
}

// FileStream is a struct used for streaming logs from a file, eg. when a test
// program writes verbose logs to a file. Intended to be used like this:
//
//	fs := s.NewFileStream("verbose")
//	defer util.Close(fs)
//	_, err := s.RunCwd(".", myTestProg, "--verbose", fs.FilePath())
//
type FileStream struct {
	id      string
	cancel  context.CancelFunc
	ctx     context.Context
	file    *os.File
	name    string
	w       io.Writer
	watcher *fsnotify.Watcher
}

// Create a log stream which uses an intermediate file, eg. for writing from a
// test program.
func (s *Step) NewFileStream(name string) *FileStream {
	w, id := s.NewLogStream()
	f, err := ioutil.TempFile("", "log")
	if err != nil {
		panic(err) // TODO(borenet): Should we return an error?
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		panic(err)
	}
	watcher.Add(f.Name())
	ctx, cancel := context.WithCancel(context.Background())
	rv := &FileStream{
		id:      id,
		cancel:  cancel,
		ctx:     ctx,
		file:    f,
		name:    name,
		w:       w,
		watcher: watcher,
	}
	s.fileStreams = append(s.fileStreams, rv)
	// Start collecting logs from the file.
	go rv.follow()
	return rv
}

// Read from the file incrementally as it is written, writing its contents to
// the step's log emitter.
func (fs *FileStream) follow() {
	buf := make([]byte, 128)
	for {
		select {
		case <-fs.ctx.Done():
			// fs.Close() was called; close and delete the file.
			if err := fs.file.Close(); err != nil {
				panic(err)
			}
			if err := os.Remove(fs.file.Name()); err != nil {
				panic(err)
			}
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
					panic(err)
				}
				if nRead > 0 {
					nWrote, err := fs.w.Write(buf[:nRead])
					if err != nil {
						panic(err)
					}
					if nWrote != nRead {
						sklog.Fatalf("Read %d bytes but wrote %d!", nRead, nWrote)
					}
				}
				if err == io.EOF {
					break
				}
			}
		case err := <-fs.watcher.Errors:
			sklog.Fatalf("fsnotify watcher error: %v", err)
		}
	}
}

// Close the FileStream, cleaning up its resources and deleting the log file.
func (fs *FileStream) Close() error {
	fs.cancel()
	return fs.watcher.Close()
}

// Return the path to the logfile used by this FileStream.
func (fs *FileStream) FilePath() string {
	return fs.file.Name()
}

// Return a context.Context associated with this Step. Any calls to exec which
// use this Context will be attached to the Step.
func (s *Step) Ctx() context.Context {
	if !s.IsRunning() {
		panic("Ctx() called on Step which is not running")
	}
	return exec.NewContext(context.Background(), func(cmd *exec.Command) error {
		name := strings.Join(append([]string{cmd.Name}, cmd.Args...), " ")
		return s.Step().Name(name).Do(func(s *Step) error {
			// Inherit env from the step unless it's explicitly provided.
			// TODO(borenet): Should we merge instead?
			if cmd.Env == nil {
				cmd.Env = s.StepProperties.Env
			}

			// Set up stdout and stderr streams.
			stdout, stdoutId := s.NewLogStream()
			if cmd.Stdout != nil {
				stdout = util.MultiWriter([]io.Writer{cmd.Stdout, stdout})
			}
			cmd.Stdout = stdout
			stderr, stderrId := s.NewLogStream()
			if cmd.Stderr != nil {
				stderr = util.MultiWriter([]io.Writer{cmd.Stderr, stderr})
			}
			cmd.Stderr = stderr

			// Collect step metadata about the command.
			d := &execData{
				Cmd: append([]string{cmd.Name}, cmd.Args...),
				Env: cmd.Env,
				Logs: map[string]string{
					"stdout": stdoutId,
					"stderr": stderrId,
				},
			}

			// Determine if any known log file streams are being used. If
			// so, start reading from them, and attach them to the execData.
			for _, fs := range s.fileStreams {
				for _, arg := range cmd.Args {
					if strings.Contains(arg, fs.FilePath()) {
						d.Logs[fs.name] = fs.id
					}
				}
			}

			// Send step metadata.
			s.Data(d)

			// Run the command.
			return exec.DefaultRun(cmd)
		})
	})
}

// httpTransport is an http.RoundTripper which wraps another http.RoundTripper
// to record data about the requests it sends.
type httpTransport struct {
	s  *Step
	rt http.RoundTripper
}

// See documentation for http.RoundTripper.
func (t *httpTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	return resp, t.s.Name(req.URL.String()).Do(func(s *Step) error {
		s.Data(req)
		var err error
		resp, err = t.rt.RoundTrip(req)
		if resp != nil {
			s.Data(resp)
		}
		return err
	})
}

// Return an http.Client which wraps the given http.Client to record data about
// the requests it sends.
func (s *Step) HttpClient(c *http.Client) *http.Client {
	if !s.IsRunning() {
		panic("HttpClient called on Step which is not running")
	}
	if c == nil {
		c = http.DefaultClient // TODO(borenet): Use backoff client?
	}
	c.Transport = &httpTransport{
		s:  s,
		rt: c.Transport,
	}
	return c
}

// RunCwd is a convenience wrapper around exec.RunCwd which runs the given
// command as a Step. It handles calls to Start() and Done() as appropriate.
// TODO(borenet): Is this really needed?
func (s *Step) RunCwd(cwd string, command ...string) (string, error) {
	return exec.RunCwd(s.Ctx(), cwd, command...)
}

// RunCommand is a convenience wrapper around exec.Run which runs the given
// Command as a Step. It handles calls to Start() and Done() as appropriate.
// TODO(borenet): Is this really needed?
func (s *Step) RunCommand(cmd *exec.Command) error {
	return exec.Run(s.Ctx(), cmd)
}
