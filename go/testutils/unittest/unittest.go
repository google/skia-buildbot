package unittest

import (
	"fmt"
	"runtime"
	"time"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/sktest"
)

// BazelOnlyTest is a function which should be called at the beginning of tests
// which should only run under Bazel (e.g. via "bazel test ...").
func BazelOnlyTest(t sktest.TestingT) {
	if !bazel.InBazelTest() {
		t.Skip("Not running Bazel tests from outside Bazel.")
	}
}

// LinuxOnlyTest is a function which should be called at the beginning of a test
// which should only run on Linux.
func LinuxOnlyTest(t sktest.TestingT) {
	if runtime.GOOS != "linux" {
		t.Skip("Not running Linux-only tests.")
	}
}

// RequiresBigTableEmulator should be called by any test case that requires the BigTable emulator.
//
// When running locally, the test case will fail if the corresponding environment variable is unset.
// When running under RBE, the first invocation of this function will start the emulator and set the
// appropriate environment variable, and any subsequent calls will reuse the emulator instance.
func RequiresBigTableEmulator(t sktest.TestingT) {
	requiresEmulator(t, emulators.BigTable, useTestSuiteSharedEmulatorInstanceUnderRBE)
}

// RequiresDatastoreEmulator should be called by any test case that requires the Datastore emulator.
//
// When running locally, the test case will fail if the corresponding environment variable is unset.
// When running under RBE, the first invocation of this function will start the emulator and set the
// appropriate environment variable, and any subsequent calls will reuse the emulator instance.
func RequiresDatastoreEmulator(t sktest.TestingT) {
	requiresEmulator(t, emulators.Datastore, useTestSuiteSharedEmulatorInstanceUnderRBE)
}

// RequiresFirestoreEmulator should be called by any test case that requires the Firestore emulator.
//
// When running locally, the test case will fail if the corresponding environment variable is unset.
// When running under RBE, the first invocation of this function will start the emulator and set the
// appropriate environment variable, and any subsequent calls will reuse the emulator instance.
func RequiresFirestoreEmulator(t sktest.TestingT) {
	requiresEmulator(t, emulators.Firestore, useTestSuiteSharedEmulatorInstanceUnderRBE)
}

// RequiresFirestoreEmulatorWithTestCaseSpecificInstanceUnderRBE is equivalent to
// RequiresFirestoreEmulator, except that under Bazel and RBE, it will always launch a new emulator
// instance that will only be visible to the current test case. After the test case finishes, the
// emulator instance will *not* be reused by any subsequent test cases, and will eventually be
// killed after the test suite finishes running.
//
// It is safe for some test cases in a test suite to call RequiresFirestoreEmulator, and for some
// others to call RequiresFirestoreEmulatorWithTestCaseSpecificInstanceUnderRBE.
//
// This should only be used in the presence of hard-to-diagnose bugs that only occur under RBE.
func RequiresFirestoreEmulatorWithTestCaseSpecificInstanceUnderRBE(t sktest.TestingT) {
	requiresEmulator(t, emulators.Firestore, useTestCaseSpecificEmulatorInstanceUnderRBE)
}

// RequiresPubSubEmulator should be called by any test case that requires the PubSub emulator.
//
// When running locally, the test case will fail if the corresponding environment variable is unset.
// When running under RBE, the first invocation of this function will start the emulator and set the
// appropriate environment variable, and any subsequent calls will reuse the emulator instance.
func RequiresPubSubEmulator(t sktest.TestingT) {
	requiresEmulator(t, emulators.PubSub, useTestSuiteSharedEmulatorInstanceUnderRBE)
}

// emulatorScopeUnderRBE indicates whether the test case requesting an emulator can reuse an
// emulator instance started by an earlier test case, or whether it requires a test case-specific
// instance not visible to any other test cases.
//
// Note that this is ignored outside of Bazel and RBE, in which case, tests will always use the
// emulator instance pointed to by the corresponding *_EMULATOR_HOST environment variable.
type emulatorScopeUnderRBE string

const (
	useTestSuiteSharedEmulatorInstanceUnderRBE  = emulatorScopeUnderRBE("useTestSuiteSharedEmulatorInstanceUnderRBE")
	useTestCaseSpecificEmulatorInstanceUnderRBE = emulatorScopeUnderRBE("useTestCaseSpecificEmulatorInstanceUnderRBE")
)

// requiresEmulator fails the test when running outside of RBE if the corresponding *_EMULATOR_HOST
// environment variable wasn't set, or starts a new emulator instance under RBE if necessary.
func requiresEmulator(t sktest.TestingT, emulator emulators.Emulator, scope emulatorScopeUnderRBE) {
	if bazel.InBazelTestOnRBE() {
		setUpEmulatorBazelRBEOnly(t, emulator, scope)
		return
	}

	// When running locally, the developer is responsible for running any necessary emulators.
	host := emulators.GetEmulatorHostEnvVar(emulator)
	if host == "" {
		t.Fatalf(`This test requires the %s emulator, which you can start with

    $ ./scripts/run_emulators/run_emulators start

and then set the environment variables it prints out.`, emulator)
	}
}

// testCaseSpecificEmulatorInstancesBazelRBEOnly keeps track of the test case-specific emulator
// instances started by the current test case. We allow at most one test case-specific instance of
// each emulator.
var testCaseSpecificEmulatorInstancesBazelRBEOnly = map[emulators.Emulator]bool{}

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
func setUpEmulatorBazelRBEOnly(t sktest.TestingT, emulator emulators.Emulator, scope emulatorScopeUnderRBE) {
	if !bazel.InBazelTestOnRBE() {
		panic("This function must only be called when running under Bazel and RBE.")
	}

	var wasEmulatorStarted bool

	switch scope {
	case useTestCaseSpecificEmulatorInstanceUnderRBE:
		// If the current test case requests a test case-specific instance of the same emulator more
		// than once, it's almost surely a bug, so we fail loudly.
		if testCaseSpecificEmulatorInstancesBazelRBEOnly[emulator] {
			t.Fatalf("A test-case specific instance of the %s emulator was already started.", emulator)
		}

		if err := emulators.StartAdHocEmulatorInstanceAndSetEmulatorHostEnvVarBazelRBEOnly(emulator); err != nil {
			t.Fatalf("Error starting a test case-specific instance of the %s emulator: %v", emulator, err)
		}
		wasEmulatorStarted = true
		testCaseSpecificEmulatorInstancesBazelRBEOnly[emulator] = true
	case useTestSuiteSharedEmulatorInstanceUnderRBE:
		// Start an emulator instance shared among all test cases. If the emulator was already started
		// by an earlier test case, then we'll reuse the emulator instance that's already running.
		var err error
		wasEmulatorStarted, err = emulators.StartEmulatorIfNotRunning(emulator)
		if err != nil {
			t.Fatalf("Error starting emulator: %v", err)
		}

		// Setting the corresponding *_EMULATOR_HOST environment variable is what effectively makes the
		// emulator visible to the test case.
		if err := emulators.SetEmulatorHostEnvVar(emulator); err != nil {
			t.Fatalf("Error setting emulator host environment variables: %v", err)
		}
	default:
		panic(fmt.Sprintf("Unknown emulatorScopeUnderRBE value: %s", scope))
	}

	// If the above code actually started the emulator, give the emulator time to boot.
	//
	// Empirically chosen: A delay of 5 seconds seems OK for all emulators; shorter delays tend to
	// cause flakes.
	if wasEmulatorStarted {
		// TODO(kjlubick) use emulator health checks instead of just sleeping
		time.Sleep(5 * time.Second)
		fmt.Println("Finished sleeping waiting for emulator to boot")
	}

	t.Cleanup(func() {
		// By unsetting any *_EMULATOR_HOST environment variables set by the current test case, we
		// ensure that any subsequent test cases only "see" the emulators they request via any of the
		// unittest.Requires*Emulator functions. This makes dependencies on emulators explicit at the
		// test case level, and makes individual test cases more self-documenting.
		if err := emulators.UnsetAllEmulatorHostEnvVars(); err != nil {
			t.Fatalf("Error while unsetting the emulator host environment variables: %v", err)
		}

		// Allow subsequent test cases to start their own test case-specific instances of the emulator.
		if scope == useTestCaseSpecificEmulatorInstanceUnderRBE {
			testCaseSpecificEmulatorInstancesBazelRBEOnly[emulator] = false
		}
	})
}
