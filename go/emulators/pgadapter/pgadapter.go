package pgadapter

import (
	"context"
	"os/exec"
	"strconv"
	"sync"

	"go.skia.org/infra/bazel/external/pgadapter_jar"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sktest"
)

var (
	isRunning      bool
	isRunningMutex sync.Mutex
)

// Require starts or verifies that a long-living instance of PGAdapter is running.
func Require(t sktest.TestingT) {
	emulators.RequireEmulator(t, emulators.PGAdapter, StartPGAdapterIfNotRunning)
}

// StartPGAdapterIfNotRunning starts the pgadapter process if it hasn't been started yet.
func StartPGAdapterIfNotRunning() (bool, error) {
	isRunningMutex.Lock()
	defer isRunningMutex.Unlock()
	if isRunning {
		return false, nil
	}
	pgAdapterJarPath, err := pgadapter_jar.Find()
	if err != nil {
		return false, skerr.Wrap(err)
	}

	cmd := exec.CommandContext(context.Background(), "java",
		"-jar", pgAdapterJarPath,
		"-s", strconv.Itoa(emulators.PGAdapterPort),
		"-c", "\"\"",
		"-p", "emulator-project",
		"-i", "test-instance",
		"-r", "autoConfigEmulator=true")

	if err := emulators.StartEmulatorCmd(cmd); err != nil {
		return false, skerr.Wrap(err)
	}
	isRunning = true
	return true, nil
}
