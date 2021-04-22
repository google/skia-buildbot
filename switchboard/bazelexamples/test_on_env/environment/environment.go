// This is an example binary that runs alongside a Bazel test. Its lifecycle is managed by the
// test_on_env rule, which launches it before running the test, and kills it after the test exits.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func main() {
	// Temporary directory created by the test_on_env rule. This environment variable is set for both
	// the environment binary and the test binary.
	envDir := os.Getenv("ENV_DIR")
	if envDir == "" {
		panic("The ENV_DIR environment variable is not set. Are we running inside a test_on_env test?")
	}

	// We must create this file to signal the test_on_env rule that the environment is ready.
	envReadyFile := os.Getenv("ENV_READY_FILE")
	if envReadyFile == "" {
		panic("The ENV_READY_FILE environment variable is not set. Are we running inside a test_on_env test?")
	}
	if err := os.WriteFile(envReadyFile, []byte{}, 0755); err != nil {
		panic(err)
	}

	// Temporary file expected by the test running alongside this binary.
	tempFile := filepath.Join(envDir, "tempfile")

	// The test will read the temp file once per second, and it will assert that its contents change
	// between each read.
	for fileContents := 1; /* iterate forever */; fileContents++ {
		time.Sleep(500 * time.Millisecond)
		if err := os.WriteFile(tempFile, []byte(fmt.Sprintf("%d\n", fileContents)), 0755); err != nil {
			panic(err)
		}
	}
}
