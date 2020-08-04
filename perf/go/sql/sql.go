package sql

import "os"

// cockroachDBEmulatorHostEnvVar is the name of the environment variable
// that points to a test instance of CockroachDB.
const cockroachDBEmulatorHostEnvVar = "COCKROACHDB_EMULATOR_HOST"

// GetCockroachDBEmulatorHost returns the connection string to connect to a
// local test instance of CockroachDB.
func GetCockroachDBEmulatorHost() string {
	ret := os.Getenv(cockroachDBEmulatorHostEnvVar)
	if ret == "" {
		ret = "localhost:26257"
	}
	return ret
}
