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

func runGCloudCmd(args ...string) error {
	gcloud, err := google_cloud_sdk.FindGcloud()
	if err != nil {
		return skerr.Wrapf(err, "finding Bazel-downloaded gcloud command")
	}

	// Add Bazel-downloaded `python3` binary to the PATH. The `gcloud` comand requires this.
	python3, err := rules_python.FindPython3()
	if err != nil {
		return skerr.Wrap(err)
	}
	python3BinaryDir := filepath.Dir(python3)
	if err := os.Setenv("PATH", fmt.Sprintf("%s:%s", python3BinaryDir, os.Getenv("PATH"))); err != nil {
		return skerr.Wrap(err)
	}

	// We intentionally do not take a context parameter because we want this instance to
	// outlive this test invocation (and be re-used by future tests).
	cmd := exec.CommandContext(context.Background(), gcloud, args...)
	if err := emulators.StartForRBE(cmd); err != nil {
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
	return nil
}
