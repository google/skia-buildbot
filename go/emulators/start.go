package emulators

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
	"time"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sktest"
)

type StartEmulator func() (bool, error)

// RequireEmulator fails the test when running outside of RBE if the corresponding *_EMULATOR_HOST
// environment variable wasn't set, or starts a new emulator instance under RBE if necessary.
// This should not be called by unit tests directly, but rather by the subpackages of this module.
func RequireEmulator(t sktest.TestingT, emulator Emulator, fn StartEmulator) {
	if bazel.InBazelTestOnRBE() {
		setUpEmulatorBazelRBEOnly(t, emulator, fn)
		return
	}

	// When running locally, the developer is responsible for running any necessary emulators.
	host := GetEmulatorHostEnvVar(emulator)
	if host == "" {
		t.Fatalf(`This test requires the %s emulator, which you can start with

    $ ./scripts/run_emulators/run_emulators start

and then set the environment variables it prints out.`, emulator)
	}
}

// setUpEmulatorBazelRBEOnly starts an instance of the given emulator, or reuses an existing
// instance if one was started by an earlier test case.
//
// If a test case-specific instance is requested, it will always start a new emulator instance,
// regardless of whether one was started by an earlier test case. The test case-specific instance
// will only be visible to the current test case (i.e. it will not be visible to any subsequent test
// cases).
//
// Test case-specific instances should only be used in the presence of hard-to-diagnose bugs that
// only occur under RBE.
func setUpEmulatorBazelRBEOnly(t sktest.TestingT, emulator Emulator, fn StartEmulator) {
	started, err := fn()
	if err != nil {
		t.Fatalf("Could not start %s Emulator: %s", emulator, err)
		return
	}
	if err = setEmulatorHostEnvVar(emulator); err != nil {
		t.Fatalf("Could not set env var for %s Emulator: %s", emulator, err)
		return
	}

	// If the above code actually started the emulator, give the emulator time to boot.
	//
	// Empirically chosen: A delay of 5 seconds seems OK for all emulators; shorter delays tend to
	// cause flakes.
	if started {
		// TODO(kjlubick) use emulator health checks instead of just sleeping
		time.Sleep(5 * time.Second)
		fmt.Println("Finished sleeping waiting for emulator to boot")
	}

	t.Cleanup(func() {
		// By unsetting any *_EMULATOR_HOST environment variables set by the current test case, we
		// ensure that any subsequent test cases only "see" the emulators they request via any of the
		// unittest.Requires*Emulator functions. This makes dependencies on emulators explicit at the
		// test case level, and makes individual test cases more self-documenting.
		if err := unsetAllEmulatorHostEnvVars(); err != nil {
			t.Fatalf("Error while unsetting the emulator host environment variables: %v", err)
		}
	})
}

// StartForRBE performs some shared-setup to starting emulators. It should not be called by anything
// other than subpackages of this package.
func StartForRBE(cmd *exec.Cmd) error {
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if bazel.InBazelTestOnRBE() {
		// Force emulator child processes to die as soon as the parent process (e.g. the Go test runner)
		// dies. If we don't do this, the emulators will continue running indefinitely after the parent
		// process dies, eventually timing out.
		//
		// Note that this is only possible under Linux. The below function call will panic under
		// non-Linux operating systems. Running emulator tests under RBE on non-Linux OSes is therefore
		// not supported. This is OK because our RBE instance is currently Linux-only. See the comments
		// in the function body for alternative approaches if we ever decide to run emulator tests under
		// RBE on other operating systems.
		cmd.SysProcAttr = makeSysProcAttrWithPdeathsigSIGKILL()
	}

	// Start the emulator.
	fmt.Printf("Starting emulator: %s %s\n", cmd.Path, cmd.Args)
	if err := cmd.Start(); err != nil {
		return skerr.Wrap(err)
	}

	// Log the emulator's exit status.
	go func() {
		err := cmd.Wait()
		if err != nil {
			fmt.Printf("Emulator %s finished with error: %v\n", cmd.Path, err)
			return
		}
		fmt.Printf("Emulator %s finished with exit status: %d\n", cmd.Path, cmd.ProcessState.Sys().(syscall.WaitStatus).ExitStatus())
	}()

	return nil
}
