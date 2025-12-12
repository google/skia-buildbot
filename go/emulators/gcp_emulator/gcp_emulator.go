package gcp_emulator

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"

	"go.skia.org/infra/bazel/external/google_cloud_sdk"
	"go.skia.org/infra/bazel/external/rules_python"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sktest"
)

func RequireBigTable(t sktest.TestingT) {
	emulators.RequireEmulator(t, emulators.BigTable, startBigTableEmulatorIfNotRunning)
}

func RequireDatastore(t sktest.TestingT) {
	emulators.RequireEmulator(t, emulators.Datastore, startDatastoreEmulatorIfNotRunning)
}

func RequireFirestore(t sktest.TestingT) {
	emulators.RequireEmulator(t, emulators.Firestore, startFirestoreEmulatorIfNotRunning)
}

func RequirePubSub(t sktest.TestingT) {
	emulators.RequireEmulator(t, emulators.PubSub, startPubSubEmulatorIfNotRunning)
}

func RequireSpanner(t sktest.TestingT) {
	emulators.RequireEmulator(t, emulators.Spanner, startSpannerEmulatorIfNotRunning)
}

var (
	isRunning      = map[string]bool{}
	isRunningMutex sync.Mutex
)

func startBigTableEmulatorIfNotRunning() (bool, error) {
	const emulator = "bigtable"
	isRunningMutex.Lock()
	defer isRunningMutex.Unlock()
	if isRunning[emulator] {
		return false, nil
	}
	err := runGCloudCmd("beta", "emulators", emulator, "start",
		fmt.Sprintf("--host-port=localhost:%d", emulators.BigTablePort),
		"--project=test-project")
	if err != nil {
		return false, skerr.Wrapf(err, "Starting pubsub emulator")
	}
	isRunning[emulator] = true
	return true, nil
}

func startDatastoreEmulatorIfNotRunning() (bool, error) {
	const emulator = "datastore"
	isRunningMutex.Lock()
	defer isRunningMutex.Unlock()
	if isRunning[emulator] {
		return false, nil
	}
	err := runGCloudCmd("beta", "emulators", emulator, "start",
		fmt.Sprintf("--host-port=localhost:%d", emulators.DataStorePort),
		"--no-store-on-disk", "--project=test-project")
	if err != nil {
		return false, skerr.Wrapf(err, "Starting pubsub emulator")
	}
	isRunning[emulator] = true
	return true, nil
}

func startFirestoreEmulatorIfNotRunning() (bool, error) {
	const emulator = "firestore"
	isRunningMutex.Lock()
	defer isRunningMutex.Unlock()
	if isRunning[emulator] {
		return false, nil
	}
	err := runGCloudCmd("beta", "emulators", emulator, "start",
		fmt.Sprintf("--host-port=localhost:%d", emulators.FirestorePort))
	if err != nil {
		return false, skerr.Wrapf(err, "Starting pubsub emulator")
	}
	isRunning[emulator] = true
	return true, nil
}

func startPubSubEmulatorIfNotRunning() (bool, error) {
	const emulator = "pubsub"
	isRunningMutex.Lock()
	defer isRunningMutex.Unlock()
	if isRunning[emulator] {
		return false, nil
	}
	err := runGCloudCmd("beta", "emulators", emulator, "start",
		fmt.Sprintf("--host-port=localhost:%d", emulators.PubSubPort),
		"--project=test-project")
	if err != nil {
		return false, skerr.Wrapf(err, "Starting pubsub emulator")
	}
	isRunning[emulator] = true
	return true, nil
}

// startSpannerEmulatorIfNotRunning starts the spanner emulator if not running.
func startSpannerEmulatorIfNotRunning() (bool, error) {
	const emulator = "spanner"
	isRunningMutex.Lock()
	defer isRunningMutex.Unlock()
	if isRunning[emulator] {
		return false, nil
	}
	err := runGCloudCmd("emulators", emulator, "start",
		fmt.Sprintf("--host-port=localhost:%d", emulators.SpannerPort),
		"--project=test-project")
	if err != nil {
		return false, skerr.Wrapf(err, "Starting spanner emulator")
	}
	isRunning[emulator] = true
	return true, nil
}

func runGCloudCmd(args ...string) error {
	gcloud, err := google_cloud_sdk.Find()
	if err != nil {
		return skerr.Wrapf(err, "finding Bazel-downloaded gcloud command")
	}

	// Add Bazel-downloaded `python3` binary to the PATH. The `gcloud` comand requires this.
	python3, err := rules_python.Find()
	if err != nil {
		return skerr.Wrap(err)
	}
	python3BinaryDir := filepath.Dir(python3)
	if err := os.Setenv("PATH", fmt.Sprintf("%s:%s", python3BinaryDir, os.Getenv("PATH"))); err != nil {
		return skerr.Wrap(err)
	}

	// If the gcloud command tries to use an interactive prompt to handle cases where e.g.
	// a particular gcloud component isn't already installed and it asks the user if they want
	// to install it, the `--quiet` flag tells gcloud to automatically use the default response
	// (e.g. go ahead and install the missing component) rather than wait for user input.
	args = append([]string{"--quiet"}, args...)

	// We intentionally do not take a context parameter because we want this instance to
	// outlive this test invocation (and be re-used by future tests).
	cmd := exec.CommandContext(context.Background(), gcloud, args...)
	if err := emulators.StartEmulatorCmd(cmd); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func StartAllIfNotRunning() error {
	if _, err := startBigTableEmulatorIfNotRunning(); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := startDatastoreEmulatorIfNotRunning(); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := startFirestoreEmulatorIfNotRunning(); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := startPubSubEmulatorIfNotRunning(); err != nil {
		return skerr.Wrap(err)
	}
	if _, err := startSpannerEmulatorIfNotRunning(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
