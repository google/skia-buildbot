// Package pubsub contains utilities for working with Cloud PubSub.
package pubsub

import (
	"os"

	"go.skia.org/infra/go/emulators"
)

// EnsureNotEmulator panics if the PubSub emulator environment variable is set.
func EnsureNotEmulator() {
	if os.Getenv(string(emulators.PubSub)) != "" {
		panic("PubSub Emulator detected. Be sure to unset the following environment variable: " + emulators.GetEmulatorHostEnvVarName(emulators.PubSub))
	}
}
