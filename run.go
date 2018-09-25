package main

import (
	"flag"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/test_automation"
)

var (
	output = flag.String("o", "", "If provided, dump a JSON blob of step data to the given file. Prints to stdout if '-' is given.")
	t      *test_automation.Run
)

func doSomething() error {
	s := t.Step().Infra().Name("say hi").Start()
	defer s.Done()
	output, err := exec.RunCwd(s.Ctx(), ".", "echo", "hi")
	if err != nil {
		s.Fail(err)
		return err
	}
	sklog.Infof("Got: %s", output)
	return nil
}

func main() {
	common.Init()

	var err error
	t, err = test_automation.New(*output)
	if err != nil {
		sklog.Fatal(err)
	}
	defer t.Done()

	if err := doSomething(); err != nil {
		sklog.Fatal(err)
	}

	if err := t.Step().Name("re-print pwd").Do(func() error {
		output, err := exec.RunCwd(t.Ctx(), ".", "pwd")
		if err != nil {
			return err
		}
		_, err = exec.RunCwd(t.Ctx(), ".", "echo", output)
		if err != nil {
			return err
		}
		return nil
	}); err != nil {
		sklog.Fatal(err)
	}

	// We should catch panics and send that information along to listeners.
	if err := t.Step().Do(func() error {
		panic("halp")
		return nil
	}); err != nil {
		sklog.Fatal(err)
	}
}
