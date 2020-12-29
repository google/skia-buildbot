package unittest

import (
	"flag"
	"os"
	"runtime"

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

// RequiresBigTableEmulator is a function that documents a unittest requires the
// BigTable Emulator and checks that the appropriate environment variable is set.
func RequiresBigTableEmulator(t sktest.TestingT) {
	s := os.Getenv("BIGTABLE_EMULATOR_HOST")
	if s == "" {
		t.Fatal(`This test requires the Bigtable emulator, which you can start with
./scripts/run_emulators/run_emulators start
and then set the environment variables it prints out.
If you need to set up the Bigtable emulator, follow the instructions at:
	https://cloud.google.com/bigtable/docs/emulator#using_the_emulator
and make sure the environment variable BIGTABLE_EMULATOR_HOST is set.
`)
	}
}

// RequiresPubSubEmulator is a function that documents a unittest requires the
// PubSub Emulator and checks that the appropriate environment variable is set.
func RequiresPubSubEmulator(t sktest.TestingT) {
	s := os.Getenv("PUBSUB_EMULATOR_HOST")
	if s == "" {
		t.Fatal(`This test requires the PubSub emulator, which you can start with

    docker run -ti -p 8010:8010 google/cloud-sdk:latest gcloud beta emulators pubsub start \
		--project test-project --host-port 0.0.0.0:8010

and then set the environment:

    export PUBSUB_EMULATOR_HOST=localhost:8010

`)
	}
}

// RequiresCockroachDB is a function that documents a unittest requires a local running version
// of the CockroachDB executable. It must be configured with the appropriate environment variable.
// For historical reasons, the environment variable uses "EMULATOR" in the name, despite it being
// an actual instance.
func RequiresCockroachDB(t sktest.TestingT) {
	s := os.Getenv("COCKROACHDB_EMULATOR_HOST")
	if s == "" {
		t.Fatal(`This test requires a local CockroachDB executable, which you can start with
./scripts/run_emulators/run_emulators start
and then set the environment variables it prints out.
If you need to install CockroachDB, follow the instructions at:
	https://www.cockroachlabs.com/docs/stable/install-cockroachdb-linux.html
and make sure the environment variable COCKROACHDB_EMULATOR_HOST is set.
`)
	}
}

// LinuxOnlyTest is a function which should be called at the beginning of a test
// which should only run on Linux.
func LinuxOnlyTest(t sktest.TestingT) {
	if runtime.GOOS != "linux" {
		t.Skip("Not running Linux-only tests.")
	}
}
