package main

/*
   Basic Task Driver example.

   Run like this:

   $ go run ./basic.go --project_id=skia-swarming-bots --task_name=basic_example -o - --local
*/

import (
	"context"
	"flag"
	"os"
	"path/filepath"

	"github.com/google/uuid"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/task_driver/go/lib/os_steps"
	"go.skia.org/infra/task_driver/go/td"
)

var (
	// Required flags for all Task Drivers.
	projectId = flag.String("project_id", "", "ID of the Google Cloud project.")
	taskId    = flag.String("task_id", "", "ID of this task. This is overridden when --local is used.")
	taskName  = flag.String("task_name", "", "Name of the task.")
	output    = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
	local     = flag.Bool("local", false, "True if running locally (as opposed to in production). This causes --task_id to be overridden.")

	// A Task Driver is, of course, welcome to define any additional flags.
)

func main() {
	// Start a new Task Driver run. The returned Context represents the
	// root-level step, from which all other steps stem. EndRun must be
	// deferred, passing in the Context returned from StartRun.
	ctx := td.StartRun(projectId, taskId, taskName, output, local)
	defer td.EndRun(ctx)

	// Technically, a Task Driver doesn't have to do anything more with
	// steps beyond this point. Any and all work would get attributed to a
	// single "root" step.

	// The root-level step is considered a failure in the case of any
	// non-recovered panic. Note that sklog.Fatal does NOT cause a panic
	// but instead uses os.Exit(). That will result in execution ending but
	// the steps not being marked as finished and no metadata sent.
	// Therefore, you should use td.Fatal instead.

	// Generally, you want to do work in sub-steps. You can choose how
	// granular you want your steps to be, but it'll be easier to debug
	// Task Drivers consisting of a larger number of smaller steps.

	// Do() is the simplest way to perform work as a sub-step of the current
	// Step. Any returned error causes the new step to be marked as failed.
	if err := td.Do(ctx, nil, func(ctx context.Context) error {
		return doSomething()
	}); err != nil {
		sklog.Error(err)
	}

	// We can add properties to steps like this:
	env := []string{
		"MYVAR=MYVAL",
	}
	props := td.Props("named infra step with env").Env(env).Infra()
	if err := td.Do(ctx, props, func(ctx context.Context) error {
		return doSomething()
	}); err != nil {
		sklog.Error(err)
	}

	// The above creates a step which is marked as an infrastructure step.
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
	if err := RunStepFunc(ctx); err != nil {
		td.Fatal(ctx, err)
	}
}

// RunStepFunc is an example of how most steps should look. It creates a step
// whose scope is the entire body of the function.
func RunStepFunc(ctx context.Context) (rvErr error) {
	// Do() is really a convenience wrapper which performs StartStep() and
	// EndStep() for you. Depending on the context, it may be cleaner
	// to use StartStep() and EndStep() directly, as in the case of a
	// step whose scope is an entire function body. In that case, we call
	// StartStep() at the beginning of the function and defer EndStep().
	// If you use a named return value, any error returned from the function
	// will be attached to the step.
	//
	// Note that EndStep() takes a pointer to an error; this is because
	// arguments to deferred functions are evaluated when they are deferred
	// (as opposed to when they are actually called), which in this case
	// would cause the error passed to EndStep() to always be nil.
	ctx = td.StartStep(ctx, td.Props("function-scoped step"))
	defer td.EndStep(ctx)

	// Function-scoped steps are the only context in which StartStep() and
	// EndStep() should be used directly. We strongly recommend against
	// the following usage pattern:
	//
	//	subStep := td.StartStep(ctx)
	//	err := doSomething()
	//	td.EndStep(subStep)
	//
	// This is wrong for a couple reasons:
	// 1. If doSomething() panics, the panic won't be correctly attributed
	//    to the sub-step.
	// 2. Storing subStep in the local scope can cause a number of mistakes,
	//    including trying to perform work before it is started or after it
	//    is marked finished (which causes a panic), or accidentally
	//    attributing work to the wrong step.
	//
	// Additionally, avoid storing a step as a member of a struct. This is a
	// recipe for the same kinds of mistakes. You should pass around steps
	// just like any other context.Context.

	// As you might have noticed, steps support nesting. This is a good way
	// to maintain high granularity of steps while being able to hide detail
	// when it's not relevant. Note that a step does not inherit the results
	// of its children; a step only fails when you call FailStep() or when
	// it catches a panic. If a sub-step failure should cause its parent
	// step to fail, then you should call FailStep() for the parent as well.
	// The Task Driver as a whole fails when the root-level step fails, or
	// when EndRun() catches a panic.
	if err := td.Do(ctx, td.Props("parent step"), func(ctx context.Context) error {
		if err := td.Do(ctx, td.Props("sub-step 1"), func(ctx context.Context) error {
			// Perform some work.
			return doSomething()
		}); err != nil {
			// If we don't return the error, the parent step doesn't
			// inherit the step failure.
			sklog.Error(err)
		}
		return td.Do(ctx, td.Props("sub-step 2"), func(ctx context.Context) error {
			// Any error produced here will be inherited by the
			// parent step.
			return doSomething()
		})
	}); err != nil {
		// The function-scoped step will not inherit the result of
		// "parent step", since we don't call FailStep().
		sklog.Error(err)
	}

	// We expect most of the work done by a Task Driver to fall into one of
	// three categories:
	//
	// 1. Subprocesses. You can pass a context.Context associated with a
	//    step to any of the Run functions in the go.skia.org/infra/go/exec
	//    package. This causes any subprocess to run as its own step, which
	//    is a sub-step of the one associated with the Context. See below and
	//    in basic_test for example code on mocking this subprocess call out.
	if _, err := exec.RunSimple(ctx, "echo helloworld"); err != nil {
		return td.FailStep(ctx, err)
	}

	// 2. HTTP requests. We provide an HttpClient() function which
	//    optionally wraps an existing http.Client and causes any HTTP
	//    request to run as a step. If you can avoid it, do not store the
	//    client; any HTTP requests become sub-steps of the step which
	//    generated the client, and it is easy to accidentally attribute
	//    requests to the wrong parent step if the client is stored.
	if _, err := td.HttpClient(ctx, nil).Get("http://www.google.com"); err != nil {
		return td.FailStep(ctx, err)
	}

	// 3. OS or filesystem interactions. We provide a library of steps which
	//    wrap the normal Go library functions so that they can be run as
	//    Steps.
	dir := filepath.Join(os.TempDir(), "task_driver_basic_example", uuid.New().String())
	if err := os_steps.MkdirAll(ctx, dir); err != nil {
		return td.FailStep(ctx, err)
	}
	// We can run steps in a defer, too!
	defer func() {
		if err := os_steps.RemoveAll(ctx, dir); err != nil {
			rvErr = td.FailStep(ctx, err)
		}
	}()
	return nil
}

// subprocessExample is accompanied by a test in basic_test.go. The test shows off how to mock out
// a call to creating a subprocess.
func subprocessExample(ctx context.Context) error {
	ctx = td.StartStep(ctx, td.Props("execute llamasay (demonstration of mocking calls)"))
	defer td.EndStep(ctx)
	if _, err := exec.RunSimple(ctx, "llamasay hello world"); err != nil {
		return td.FailStep(ctx, err)
	}

	if _, err := exec.RunSimple(ctx, "bearsay good night moon"); err != nil {
		return td.FailStep(ctx, err)
	}
	return nil
}

// doSomething is a dummy function used to take the place of actual work in
// this example.
func doSomething() error {
	return nil
}
