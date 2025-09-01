package gcp_emulator

import (
	"context"
	"fmt"
	"os/exec"
	"sync"

	"go.skia.org/infra/bazel/external/google_cloud_sdk"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sktest"
)

func RequireBigTable(t sktest.TestingT) {
	// NOOP
	// TODO(jcgregorio) Remove all references to bigtable.
}

func RequireDatastore(t sktest.TestingT) {
	// NOOP
	// TODO(jcgregorio) Remove all references to datastore.
}

func RequireFirestore(t sktest.TestingT) {
	// NOOP
	// TODO(jcgregorio) Remove all references to firestore.
}

func RequirePubSub(t sktest.TestingT) {
	emulators.RequireEmulator(t, emulators.PubSub, startPubSubEmulatorIfNotRunning)
}

var (
	isRunning      = map[string]bool{}
	isRunningMutex sync.Mutex
)

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

	// gcloud on linux x86_64 comes with its own python runtime, so we don't need Python on the PATH.

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
	if _, err := startPubSubEmulatorIfNotRunning(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
