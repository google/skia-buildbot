// Package emulators contains functions to start and stop emulators, and utilities to work with the
// various *_EMULATOR_HOST environment variables.
//
// TODO(lovisolo): Make the start/stop operations work on Bazel and RBE.
package emulators

// This package uses "os/exec" as opposed to "go.skia.org/infra/go/exec" in order to avoid the
// following circular dependency:
//
//   //go/exec/exec_test.go -> //go/testutils/unittest/unittest.go -> //go/emulators/emulators.go

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"

	"go.skia.org/infra/go/skerr"
)

// Emulator represents a Google Cloud emulator, a test-only CockroachDB server, etc.
type Emulator string

const (
	// BigTable represents a Google Cloud BigTable emulator.
	BigTable = Emulator("bigtable")

	// CockroachDB represents a test-only CockroachDB instance.
	CockroachDB = Emulator("cockroachdb")

	// Datastore represents a Google Cloud Datastore emulator.
	Datastore = Emulator("datastore")

	// Firestore represents a Google Cloud Firestore emulator.
	Firestore = Emulator("firestore")

	// PubSub represents a Google Cloud PubSub emulator.
	PubSub = Emulator("pubsub")
)

// AllEmulators contains a list of all known emulators.
var AllEmulators = []Emulator{BigTable, CockroachDB, Datastore, Firestore, PubSub}

// emulatorInfo holds the information necessary to start an emulator and manage its lifecycle.
type emulatorInfo struct {
	cmd    string // Command and arguments to start the emulator on a developer workstation.
	envVar string // Name of the emulator's environment variable, e.g. "FOO_EMULATOR_HOST".
	port   int    // The emulator's TCP port when running locally. Ignored under RBE.
}

// cachedEmulatorInfos is populated by getEmulatorInfo() on the first call. Do not use directly.
var cachedEmulatorInfos = map[Emulator]emulatorInfo{}

// getEmulatorInfo builds and returns an emulatorInfo struct for the given emulator. The struct's
// contents will depend on whether we are running on Bazel and RBE or not. All structs are computed
// once and cached. Subsequent calls will return the same struct given the same input.
func getEmulatorInfo(emulator Emulator) emulatorInfo {
	if len(cachedEmulatorInfos) == 0 {
		// Read the CockroachDB storage directory from an environment variable, or create a temp dir.
		cockroachDbStoreDir := os.Getenv("COCKROACHDB_EMULATOR_STORE_DIR")
		if cockroachDbStoreDir == "" {
			var err error
			cockroachDbStoreDir, err = ioutil.TempDir("", "crdb-emulator-*")
			if err != nil {
				panic("Error while creating temporary directory: " + skerr.Wrap(err).Error())
			}
		}

		cachedEmulatorInfos = map[Emulator]emulatorInfo{
			BigTable: {
				cmd:    "gcloud beta emulators bigtable start --host-port=localhost:%d --project=test-project",
				envVar: "BIGTABLE_EMULATOR_HOST",
				port:   8892,
			},
			CockroachDB: {
				cmd:    fmt.Sprintf("cockroach start-single-node --insecure --listen-addr=localhost:%%d --store=%s", cockroachDbStoreDir),
				envVar: "COCKROACHDB_EMULATOR_HOST",
				port:   8895,
			},
			Datastore: {
				cmd:    "gcloud beta emulators datastore start --no-store-on-disk --host-port=localhost:%d --project=test-project",
				envVar: "DATASTORE_EMULATOR_HOST",
				port:   8891,
			},
			Firestore: {
				cmd:    "gcloud beta emulators firestore start --host-port=localhost:%d",
				envVar: "FIRESTORE_EMULATOR_HOST",
				port:   8894,
			},
			PubSub: {
				cmd:    "gcloud beta emulators pubsub start --host-port=localhost:%d --project=test-project",
				envVar: "PUBSUB_EMULATOR_HOST",
				port:   8893,
			},
		}
	}

	emulatorInfo, ok := cachedEmulatorInfos[emulator]
	if !ok {
		panic("Unknown emulator: " + emulator)
	}
	return emulatorInfo
}

// GetEmulatorHostEnvVar returns the contents of the *_EMULATOR_HOST environment variable corresponding to
// the given emulator, or the empty string if the environment variable is unset.
func GetEmulatorHostEnvVar(emulator Emulator) string {
	return os.Getenv(getEmulatorInfo(emulator).envVar)
}

// GetEmulatorHostEnvVarName returns the name of the *_EMULATOR_HOST environment variable corresponding
// to the given emulator.
func GetEmulatorHostEnvVarName(emulator Emulator) string {
	return getEmulatorInfo(emulator).envVar
}

// StartEmulator starts an emulator.
func StartEmulator(emulator Emulator) error {
	emulatorInfo := getEmulatorInfo(emulator)
	programAndArgs := strings.Split(fmt.Sprintf(emulatorInfo.cmd, emulatorInfo.port), " ")
	cmd := exec.Command(programAndArgs[0], programAndArgs[1:]...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// StartAllEmulators starts all known emulators.
func StartAllEmulators() error {
	for _, emulator := range AllEmulators {
		if err := StartEmulator(emulator); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

var emulatorProcsToKill = []*regexp.Regexp{
	regexp.MustCompile("[g]cloud\\.py"),
	regexp.MustCompile("[c]loud_datastore_emulator"),
	regexp.MustCompile("[C]loudDatastore.jar"),
	regexp.MustCompile("[c]btemulator"),
	regexp.MustCompile("[c]loud-pubsub-emulator"),
	regexp.MustCompile("[c]loud-firestore-emulator"),
	regexp.MustCompile("[c]ockroach"),
}

// StopAllEmulators stops all known emulators.
func StopAllEmulators() error {
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
				if err := exec.Command("kill", id).Run(); err != nil {
					return skerr.Wrap(err)
				}
				delete(procs, desc)
			}
		}
	}
	return nil
}
