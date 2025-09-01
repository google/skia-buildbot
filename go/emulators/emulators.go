// Package emulators contains functions to start and stop emulators, and utilities to work with the
// various *_EMULATOR_HOST environment variables.
//
// Unless otherwise specified, all functions in this package assume that there will be at most one
// instance of each emulator running at any given time.
package emulators

// This package uses "os/exec" as opposed to "go.skia.org/infra/go/exec" in order to avoid the
// following circular dependency:
//
//   //go/exec/exec_test.go -> //go/testutils/unittest/unittest.go -> //go/emulators/emulators.go

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"go.skia.org/infra/bazel/go/bazel"
	"go.skia.org/infra/go/skerr"
)

// Emulator represents a Google Cloud emulator, a test-only CockroachDB server, etc.
type Emulator string

const (
	// CockroachDB represents a test-only CockroachDB instance.
	CockroachDB       = Emulator("CockroachDB")
	CockroachDBEnvVar = "COCKROACHDB_EMULATOR_HOST"
	CockroachDBPort   = 8895

	// PubSub represents a Google Cloud PubSub emulator.
	PubSub       = Emulator("PubSub")
	PubSubEnvVar = "PUBSUB_EMULATOR_HOST"
	PubSubPort   = 8893
)

var AllEmulators = []Emulator{CockroachDB, PubSub}

// GetEmulatorHostEnvVar returns the contents of the *_EMULATOR_HOST environment variable
// corresponding to the given emulator, or the empty string if the environment variable is unset.
func GetEmulatorHostEnvVar(emulator Emulator) string {
	return os.Getenv(GetEmulatorHostEnvVarName(emulator))
}

// GetEmulatorHostEnvVarName returns the name of the *_EMULATOR_HOST environment variable
// corresponding to the given emulator.
func GetEmulatorHostEnvVarName(emulator Emulator) string {
	switch emulator {
	case CockroachDB:
		return CockroachDBEnvVar
	case PubSub:
		return PubSubEnvVar
	default:
		panic("Unknown emulator " + emulator)
	}
}

func setEmulatorHostEnvVar(emulator Emulator) error {
	envVar := GetEmulatorHostEnvVarName(emulator)
	var port int
	switch emulator {
	case CockroachDB:
		port = CockroachDBPort
	case PubSub:
		port = PubSubPort
	default:
		panic("Unknown emulator " + emulator)
	}
	return skerr.Wrap(os.Setenv(envVar, fmt.Sprintf("localhost:%d", port)))
}

func unsetAllEmulatorHostEnvVars() error {
	vars := []string{CockroachDBEnvVar, PubSubEnvVar}
	for _, envVar := range vars {
		if err := os.Setenv(envVar, ""); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

var emulatorProcsToKill = []*regexp.Regexp{
	regexp.MustCompile("[g]cloud\\.py"),
	regexp.MustCompile("[c]loud-pubsub-emulator"),
	regexp.MustCompile("[c]ockroach"),
}

// StopAllEmulators gracefully terminates all known emulators.
func StopAllEmulators() error {
	signal := "SIGTERM"
	if bazel.InBazelTestOnRBE() {
		// Under Bazel and RBE, we don't need graceful termination because the RBE containers are
		// ephemeral. Killing the emulators with SIGKILL is faster and simpler.
		signal = "SIGKILL"
	}
	return stopAllEmulators(signal)
}

// ForceStopAllEmulators immediately terminates all known emulators with SIGKILL.
func ForceStopAllEmulators() error {
	return stopAllEmulators("SIGKILL")
}

func stopAllEmulators(signal string) error {
	// List all processes.
	psCmd := exec.Command("ps", "aux")
	var psOut bytes.Buffer
	psCmd.Stdout = &psOut
	if err := psCmd.Run(); err != nil {
		return skerr.Wrap(err)
	}

	// Parse the output of the previous command.
	lines := strings.Split(psOut.String(), "\n")
	procs := make(map[string]string, len(lines))
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) < 11 {
			continue
		}
		procs[line] = fields[1]
	}

	// Kill each matching process.
	for _, re := range emulatorProcsToKill {
		for desc, id := range procs {
			if re.MatchString(desc) {
				if err := exec.Command("kill", "-s", signal, id).Run(); err != nil {
					return skerr.Wrap(err)
				}
				delete(procs, desc)
			}
		}
	}

	return nil
}
