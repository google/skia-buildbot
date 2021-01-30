package unittest

import (
	"flag"
	"runtime"
	"strings"
	"time"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/sktest"
)

const (
	SMALL_TEST  = "small"
	MEDIUM_TEST = "medium"
	LARGE_TEST  = "large"
	MANUAL_TEST = "manual"
)

var (
	small         = flag.Bool(SMALL_TEST, false, "Whether or not to run small tests.")
	medium        = flag.Bool(MEDIUM_TEST, false, "Whether or not to run medium tests.")
	large         = flag.Bool(LARGE_TEST, false, "Whether or not to run large tests.")
	manual        = flag.Bool(MANUAL_TEST, false, "Whether or not to run manual tests.")
	uncategorized = flag.Bool("uncategorized", false, "Only run uncategorized tests.")

	// DEFAULT_RUN indicates whether the given test type runs by default
	// when no filter flag is specified.
	DEFAULT_RUN = map[string]bool{
		SMALL_TEST:  true,
		MEDIUM_TEST: true,
		LARGE_TEST:  true,
		MANUAL_TEST: true,
	}

	TIMEOUT_SMALL  = "4s"
	TIMEOUT_MEDIUM = "15s"
	TIMEOUT_LARGE  = "4m"
	TIMEOUT_MANUAL = TIMEOUT_LARGE

	TIMEOUT_RACE = "5m"

	// TEST_TYPES lists all of the types of tests.
	TEST_TYPES = []string{
		SMALL_TEST,
		MEDIUM_TEST,
		LARGE_TEST,
		MANUAL_TEST,
	}
)

// ShouldRun determines whether the test should run based on the provided flags.
func ShouldRun(testType string) bool {
	if *uncategorized {
		return false
	}

	// Fallback if no test filter is specified.
	if !*small && !*medium && !*large && !*manual {
		return DEFAULT_RUN[testType]
	}

	switch testType {
	case SMALL_TEST:
		return *small
	case MEDIUM_TEST:
		return *medium
	case LARGE_TEST:
		return *large
	case MANUAL_TEST:
		return *manual
	}
	return false
}

// SmallTest is a function which should be called at the beginning of a small
// test: A test (under 2 seconds) with no dependencies on external databases,
// networks, etc.
func SmallTest(t sktest.TestingT) {
	if !ShouldRun(SMALL_TEST) {
		t.Skip("Not running small tests.")
	}
}

// MediumTest is a function which should be called at the beginning of an
// medium-sized test: a test (2-15 seconds) which has dependencies on external
// databases, networks, etc.
func MediumTest(t sktest.TestingT) {
	if !ShouldRun(MEDIUM_TEST) {
		t.Skip("Not running medium tests.")
	}
}

// LargeTest is a function which should be called at the beginning of a large
// test: a test (> 15 seconds) with significant reliance on external
// dependencies which makes it too slow or flaky to run as part of the normal
// test suite.
func LargeTest(t sktest.TestingT) {
	if !ShouldRun(LARGE_TEST) {
		t.Skip("Not running large tests.")
	}
}

// ManualTest is a function which should be called at the beginning of tests
// which shouldn't run on the bots due to excessive running time, external
// requirements, etc. These only run when the --manual flag is set.
func ManualTest(t sktest.TestingT) {
	// Find the caller's file name.
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("runtime.Caller(0) failed")
	}
	file := thisFile
	for skip := 0; file == thisFile; skip++ {
		var ok bool
		_, file, _, ok = runtime.Caller(skip)
		if !ok {
			t.Fatalf("runtime.Caller(%d) failed", skip)
		}
	}

	// Force the naming convention expected by our custom go_test Bazel macro.
	if !strings.HasSuffix(file, "_manual_test.go") {
		t.Fatalf(`Manual tests must be placed in files ending with "_manual_test.go", was: "%s"`, file)
	}

	if !ShouldRun(MANUAL_TEST) {
		t.Skip("Not running manual tests.")
	}
}

// FakeExeTest masks a test from the uncategorized tests check. See executil.go for
// more on what FakeTests are used for.
func FakeExeTest(t sktest.TestingT) {
	if *uncategorized {
		t.Skip(`This is to appease the "uncategorized tests" check`)
	}
}

// BazelOnlyTest is a function which should be called at the beginning of tests
// which should only run under Bazel (e.g. via "bazel test ...").
func BazelOnlyTest(t sktest.TestingT) {
	if !bazel.InBazel() {
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

// RequiresBigTableEmulator is a function that documents a unittest requires the
// BigTable Emulator and checks that the appropriate environment variable is set.
func RequiresBigTableEmulator(t sktest.TestingT) {
	if bazel.InRBE() {
		setUpEmulatorBazelRBEOnly(t, emulators.BigTable)
		return
	}

	host := emulators.GetEmulatorHostEnvVar(emulators.BigTable)
	if host == "" {
		t.Fatalf(`This test requires the Bigtable emulator, which you can start with
	./scripts/run_emulators/run_emulators start
	and then set the environment variables it prints out.
	If you need to set up the Bigtable emulator, follow the instructions at:
		https://cloud.google.com/bigtable/docs/emulator#using_the_emulator
	and make sure the environment variable %s is set.
`, emulators.GetEmulatorHostEnvVarName(emulators.BigTable))
	}
}

// RequiresCockroachDB is a function that documents a unittest requires a local running version
// of the CockroachDB executable. It must be configured with the appropriate environment variable.
// For historical reasons, the environment variable uses "EMULATOR" in the name, despite it being
// an actual instance.
func RequiresCockroachDB(t sktest.TestingT) {
	if bazel.InRBE() {
		setUpEmulatorBazelRBEOnly(t, emulators.CockroachDB)
		return
	}

	host := emulators.GetEmulatorHostEnvVar(emulators.CockroachDB)
	if host == "" {
		t.Fatalf(`This test requires a local CockroachDB executable, which you can start with
	./scripts/run_emulators/run_emulators start
	and then set the environment variables it prints out.
	If you need to install CockroachDB, follow the instructions at:
		https://www.cockroachlabs.com/docs/stable/install-cockroachdb-linux.html
	and make sure the environment variable %s is set.
`, emulators.GetEmulatorHostEnvVarName(emulators.CockroachDB))
	}
}

// RequiresDatastoreEmulator is a function that documents a unittest requires the
// Datastore emulator and checks that the appropriate environment variable is set.
func RequiresDatastoreEmulator(t sktest.TestingT) {
	if bazel.InRBE() {
		setUpEmulatorBazelRBEOnly(t, emulators.Datastore)
		return
	}

	host := emulators.GetEmulatorHostEnvVar(emulators.Datastore)
	if host == "" {
		t.Fatalf(`This test requires the Datastore emulator, which you can start with
	./scripts/run_emulators/run_emulators start
	and then set the environment variables it prints out.
`)
	}
}

// RequiresFirestoreEmulator is a function that documents a unittest requires the
// Firestore emulator and checks that the appropriate environment variable is set.
func RequiresFirestoreEmulator(t sktest.TestingT) {
	if bazel.InRBE() {
		setUpEmulatorBazelRBEOnly(t, emulators.Firestore)
		return
	}

	host := emulators.GetEmulatorHostEnvVar(emulators.Firestore)
	if host == "" {
		t.Fatalf(`This test requires the Firestore emulator, which you can start with
	./scripts/run_emulators/run_emulators start
	and then set the environment variables it prints out.

	# If you need to set up the Firestore emulator:
	gcloud beta emulators firestore start
	# The above will install the emulator and fail with an error like:
	#   [firestore] Error trying to exec /path/to/cloud-firestore-emulator.jar
	# See b/134379774
	chmod +x /path/to/cloud-firestore-emulator.jar

	# If you want to start only the Firestore emulator, the default params
	# try to use IPv6, which doesn't work great for our clients, so we need to start
	# it manually like:
	/path/to/cloud-firestore-emulator.jar --host=localhost --port=8894

	# Once the emulator is running, we need to run the following in the terminal
	# that we are running the tests in:
	export %s=localhost:8894
`, emulators.GetEmulatorHostEnvVarName(emulators.Firestore))
	}
}

// RequiresPubSubEmulator is a function that documents a unittest requires the
// PubSub Emulator and checks that the appropriate environment variable is set.
func RequiresPubSubEmulator(t sktest.TestingT) {
	if bazel.InRBE() {
		setUpEmulatorBazelRBEOnly(t, emulators.PubSub)
		return
	}

	host := emulators.GetEmulatorHostEnvVar(emulators.PubSub)
	if host == "" {
		t.Fatalf(`This test requires the PubSub emulator, which you can start with

			docker run -ti -p 8010:8010 google/cloud-sdk:latest gcloud beta emulators pubsub start \
			--project test-project --host-port 0.0.0.0:8010

	and then set the environment:

			export %s=localhost:8010
`, emulators.GetEmulatorHostEnvVarName(emulators.PubSub))
	}
}

func setUpEmulatorBazelRBEOnly(t sktest.TestingT, emulator emulators.Emulator) {
	if !bazel.InRBE() {
		panic("This function must only be called when running under Bazel and RBE.")
	}

	// We only start each emulator once per test suite. If the emulator was already started by an
	// earlier test case, then we'll reuse the emulator instance that's already running.
	ok, err := emulators.StartEmulatorIfNotRunning(emulator)
	if err != nil {
		t.Fatalf("Error starting emulator: %v", err)
	}

	// If the above function call actually started the emulator, give the emulator time to boot.
	//
	// Empirically chosen: A delay of 3 seconds seems OK for all emulators; shorter delays tend to
	// cause flakes.
	if ok {
		time.Sleep(3 * time.Second)
	}

	// Setting the corresponding *_EMULATOR_HOST environment variable is what effectively makes the
	// emulator visible to the test case.
	if err := emulators.SetEmulatorHostEnvVar(emulator); err != nil {
		t.Fatalf("Error setting emulator host environment variables: %v", err)
	}

	t.Cleanup(func() {
		// By unsetting any *_EMULATOR_HOST environment variables set by the current test case, we
		// ensure that any subsequent test cases only "see" the emulators they request via any of the
		// unittest.Requires*Emulator functions. This makes dependencies on emulators explicit at the
		// test case level, and makes individual test cases more self-documenting.
		if err := emulators.UnsetAllEmulatorHostEnvVars(); err != nil {
			t.Fatalf("Error while unsetting the emulator host environment variables: %v", err)
		}
	})
}
