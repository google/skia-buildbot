package cockroachdb_instance

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"sync"

	"go.skia.org/infra/bazel/external/cockroachdb"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sktest"
)

// Require starts or verifies that a long-living instance of CockroachDB is running.
//
// When running locally, the test case will fail if the corresponding environment variable is unset.
// When running under RBE, the first invocation of this function will start the emulator and set the
// appropriate environment variable, and any subsequent calls will reuse the emulator instance.
func Require(t sktest.TestingT) {
	emulators.RequireEmulator(t, emulators.CockroachDB, StartCockroachDBIfNotRunning)
}

var (
	isRunning      bool
	isRunningMutex sync.Mutex
)

// StartCockroachDBIfNotRunning returns true if it successfully started CockroachDB, false (and
// maybe an error) if it did not (possibly because it was already running).
func StartCockroachDBIfNotRunning() (bool, error) {
	isRunningMutex.Lock()
	defer isRunningMutex.Unlock()
	if isRunning {
		return false, nil
	}
	cockroachExe, err := cockroachdb.FindCockroach()
	if err != nil {
		return false, skerr.Wrapf(err, "finding Bazel-downloaded cockroach command")
	}

	// Read the CockroachDB storage directory from an environment variable, or create a temp dir.
	cockroachDBStoreDir := os.Getenv("COCKROACHDB_EMULATOR_STORE_DIR")
	if cockroachDBStoreDir == "" {
		cockroachDBStoreDir, err = os.MkdirTemp("", "crdb-emulator-*")
		if err != nil {
			return false, skerr.Wrapf(err, "setting up temp directory")
		}
	}

	// We intentionally do not take a context parameter because we want this instance to
	// outlive this test invocation (and be re-used by future tests).
	cmd := exec.CommandContext(context.Background(), cockroachExe,
		"start-single-node", "--insecure",
		fmt.Sprintf("--listen-addr=localhost:%d", emulators.CockroachDBPort),
		"--store="+cockroachDBStoreDir,
	)
	if err := emulators.StartForRBE(cmd); err != nil {
		return false, skerr.Wrap(err)
	}
	isRunning = true
	return true, nil
}
