// This app runs alongside a Bazel test and hooks up a local connection to the
// switchboard.
package main

import (
	"context"
	"encoding/json"
	"os"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/switchboard/go/lease"
)

func main() {
	// Temporary directory created by the test_on_env rule. This environment variable is set for both
	// the environment binary and the test binary.
	envDir := os.Getenv("ENV_DIR")
	if envDir == "" {
		sklog.Fatal("The ENV_DIR environment variable is not set. Are we running inside a test_on_env test?")
	}

	resp, err := httputils.DefaultClientConfig().Client().Get("https://switchboard.skia.org/lease")
	if err != nil {
		sklog.Fatal(err)
	}

	var lease lease.Lease

	if err := json.NewDecoder(resp.Body).Decode(&lease); err != nil {
		sklog.Fatal(err)
	}

	ctx := context.Background()

	// We must create this file to signal the test_on_env rule that the environment is ready.
	envReadyFile := os.Getenv("ENV_READY_FILE")
	if envReadyFile == "" {
		sklog.Fatal("The ENV_READY_FILE environment variable is not set. Are we running inside a test_on_env test?")
	}
	if err := os.WriteFile(envReadyFile, []byte{}, 0755); err != nil {
		sklog.Fatal(err)
	}

	// Wait for exec to exit/abort.
}
