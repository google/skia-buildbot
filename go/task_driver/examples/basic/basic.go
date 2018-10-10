package main

/*
   Basic Task Driver example.

   Run like this:

   $ go run ./basic.go --logtostderr --project_id=skia-swarming-bots --task_name=basic_example -o - --local
*/

import (
	"errors"
	"flag"
	"os"
	"path/filepath"

	"github.com/pborman/uuid"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/task_driver"
	"go.skia.org/infra/go/task_driver/lib/os_steps"
)

var (
	// Required flags for all TaskDrivers.
	projectId = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId    = flag.String("task_id", "", "ID of this task.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	output    = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
	local     = flag.Bool("local", false, "True if running locally (as opposed to in production)")
)

func main() {
	// Initialize the Task Driver framework. The return value is the
	// root-level Step, from which all other Steps stem. Note that each
	// Step must be marked Done(), including any error which might have
	// occurred. If an error is passed to Done(), then the Step is marked
	// as failed.
	s := task_driver.MustInit(projectId, taskId, taskName, output, local)
	defer s.Done(nil)

	// Technically, a Task Driver doesn't have to do anything more with
	// Steps beyond this point. Any and all work would get attributed to a
	// single "root" Step. The above call to s.Done() uses a nil error; if
	// you wanted to do this more correctly you'd declare an error before
	// that line and pass a pointer to it to s.Done(), then set the error
	// to indicate that the root step failed, eg.
	//
	//	var err error
	//	defer s.Done(&err)
	//
	//	// Do some work, ensuring that you set err as appropriate.
	//	err = doSomeWork()
	//
	// Alternatively, the root-level step is considered a failure in the
	// case of any non-recovered panic.

	// Generally, you want to do work in sub-Steps. You can choose how
	// granular you want your steps to be, but it'll be easier to debug
	// Task Drivers consisting of a larger number of smaller Steps.
	// Note that you need to explicitly Start() the step.
	step1 := s.Step().Start()
	err := doSomething()
	step1.Done(&err)

	// We can add properties to steps like this:
	env := []string{
		"MYVAR=MYVAL",
	}
	step2 := s.Step().Infra().Env(env).Name("named infra step with env").Start()
	err = doSomething()
	step2.Done(&err)

	// The above creates a Step which is marked as an infrastructure step.
	// This has no effect on how the Task Driver runs, but it allows us to
	// separate different types of failures (eg. transient network errors
	// vs actual test failures) and place blame correctly.
	//
	// The above step also has a name, which is only used for display, and
	// an environment. The environment is only applied to subprocesses (ie.
	// using the exec package); we do not modify this process' environment.
	// Notably, if you add a new entry to PATH, the exec package won't be
	// able to find executables in that new entry, unless you also export
	// the new PATH via os.Setenv() or provide the absolute path to the
	// executable to the exec package. We recommend the latter, since it is
	// more easily auditable.

	// Please see docs for RunStepFunc.
	if err := RunStepFunc(s); err != nil {
		sklog.Fatal(err)
	}
}

// RunStepFunc is an example of how most Steps should look. It creates a Step
// whose scope is the entire body of the function.
func RunStepFunc(s *task_driver.Step) (err error) {
	// Generally Step.Done() should be deferred, as it is in this example.
	// If you use a named return value, any error returned from the function
	// will be attached to the Step.
	s = s.Step().Start()
	defer s.Done(&err)

	// As you might have noticed, Steps support nesting. This is a good way
	// to maintain high granularity of Steps while being able to hide detail
	// when it's not relevant. Note that a Step does not inherit the results
	// of its children; a Step only fails when you pass a non-nil error to
	// Done() or when it catches a panic. If a sub-step failure should cause
	// its parent step to fail, then you should pass the error to
	// parent.Done() as well.
	substep := s.Step().Start()
	someOtherError := errors.New("sub-step failed")
	substep.Done(&someOtherError)

	// The function-level Step will not inherit the result of substep, since
	// we didn't return it.

	// For convenience, Step also provides a Do() method which takes care of
	// calling Start() and Done() for you:
	if err := s.Step().Name("helper").Do(func(s *task_driver.Step) error {
		// Note that the current step is passed in, so we can still
		// create sub-steps in this context.
		// Any error returned by this func will be associated with the
		// "helper" step.
		return doSomething()
	}); err != nil {
		// As described above, returning the error (which caused the
		// "helper" step to fail) will cause the function-level Step
		// to also fail.
		return err
	}

	// We expect most of the work done by a Task Driver to fall into one of
	// three categories:
	//
	// 1. Subprocesses. Step provides a Ctx() method which may be passed
	//    to any of the Run functions in the go.skia.org/infra/go/exec
	//    package. This causes any subprocess to run as its own Step, which
	//    is a sub-Step of s. If you can avoid it, do not store the Context;
	//    any subprocesses become sub-Steps of the Step which generated the
	//    Context, and it is easy to accidentally attribute subprocesses to
	//    the wrong parent Step if the Context is stored.
	_, err = exec.RunSimple(s.Ctx(), "echo helloworld")
	if err != nil {
		return err
	}

	// 2. HTTP requests. Step provides an HttpClient() method which
	//    optionally wraps an existing http.Client and causes any HTTP
	//    request to run as a Step. If you can avoid it, do not store the
	//    Client; any HTTP requests become sub-Steps of the Step which
	//    generated the Client, and it is easy to accidentally attribute
	//    requests to the wrong parent Step if the Client is stored.
	httpClient := s.HttpClient(nil)
	_, err = httpClient.Get("http://www.google.com")
	if err != nil {
		return err
	}

	// 3. OS or filesystem interactions. We provide a library of Steps which
	//    wrap the normal Go library functions so that they can be run as
	//    Steps.
	dir := filepath.Join(os.TempDir(), "task_driver_basic_example", uuid.New())
	if err := os_steps.MkdirAll(s, dir); err != nil {
		return err
	}
	// We can run Steps in a defer, too!
	defer func() {
		err = os_steps.RemoveAll(s, dir)
	}()
	return nil
}

// doSomething is a dummy function used to take the place of actual work in
// this example.
func doSomething() error {
	return nil
}
