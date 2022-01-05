package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

func main() {
	var (
		testBin    = flag.String("test_bin", "", "Path to the test binary")
		envBin     = flag.String("env_bin", "", "Path to the environment binary")
		readyCheck = flag.Duration("ready_check", 5*time.Second, "Wait up to this long for the environment to be ready")
	)
	flag.Parse()
	if *testBin == "" || *envBin == "" {
		fatalf("Must provide --test_bin and --env_bin")
	}

	// For some unknown reason, Go tests fail with "fork/exec [...]: no such file or directory" when
	// invoked from this script via $TEST_BIN, which holds the path to a symlink created by Bazel.
	// Invoking the test binary via its real path, as opposed to a symlink, prevents this error.
	// We need to be sure to use filepath.EvalSymlinks and not os.ReadLink because the latter will
	// only resolve the first symlink.
	var err error
	*testBin, err = filepath.EvalSymlinks(*testBin)
	if err != nil {
		fatalf("Could not resolve symlinks to %s: %s", *testBin, err)
	}
	*envBin, err = filepath.EvalSymlinks(*envBin)
	if err != nil {
		fatalf("Could not resolve symlinks to %s: %s", *envBin, err)
	}

	fmt.Printf("TestBin %s\n", *testBin)
	fmt.Printf("EnvBin %s\n", *envBin)

	envDir := filepath.Join(os.Getenv("TEST_TMPDIR"), "envdir")
	if err := os.MkdirAll(envDir, 0755); err != nil {
		fatalf("Could not create %s: %s", envDir, err)
	}
	envReadyFile := filepath.Join(envDir, "ready")

	// Run both of the binaries with a copy of the visible environment variables plus these two.
	env := os.Environ()
	env = append(env, "ENV_DIR="+envDir, "ENV_READY_FILE="+envReadyFile)
	fmt.Printf("TEST_TMPDIR: %s\n", os.Getenv("TEST_TMPDIR"))
	fmt.Printf("ENV_DIR: %s\n", envDir)
	fmt.Printf("ENV_READY_FILE: %s\n", envReadyFile)

	// We do not want any child processes to outlive this one.
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	envCmd := exec.CommandContext(ctx, *envBin)
	envCmd.Env = env
	if err := envCmd.Start(); err != nil {
		fatalf("Could not run env binary %s: %s", *envBin, err)
	}
	fmt.Printf("Environment started with PID %d\n", envCmd.Process.Pid)
	fmt.Printf("Waiting up to %s for environment to be ready\n", *readyCheck)

	startTime := time.Now()
	tck := time.NewTicker(100 * time.Millisecond)
	ready := false
	for startTime.Sub(time.Now()) < *readyCheck {
		<-tck.C
		if _, err := os.Stat(envReadyFile); err == nil {
			ready = true
			break // The file exists, we are done
		}
	}
	tck.Stop()
	if !ready {
		fatalf("Timed out while waiting for environment to be ready")
	}

	// The environment is set up (and probably still running), we can finally run the tests
	// If *testBin refers to a bash script, but does not have a shebang as the very first line,
	// there will be a mysterious "fork/exec ... exec format error"
	testCmd := exec.CommandContext(ctx, *testBin)
	testCmd.Env = env
	testCmd.Stdout = os.Stdout
	testCmd.Stderr = os.Stderr
	if err := testCmd.Start(); err != nil {
		fatalf("Could not run test binary %s: %s", *testBin, err)
	}

	err = testCmd.Wait()
	fmt.Printf("Test finished: %v\n", err)

	// Tear down the environment. Send SIGTERM and give it a moment to gracefully shutdown before
	// this process exits.
	err = envCmd.Process.Signal(syscall.SIGTERM)
	fmt.Printf("Shutting down environment: %v\n", err)
	time.Sleep(100 * time.Millisecond)

	// forward the test exit code to Bazel
	os.Exit(testCmd.ProcessState.ExitCode())
}

func fatalf(format string, args ...interface{}) {
	// Ensure there is a newline at the end of the fatal message.
	format = strings.TrimSuffix(format, "\n") + "\n"
	fmt.Printf(format, args...)
	os.Exit(1)
}
