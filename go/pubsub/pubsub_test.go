// Package pubsub contains utilities for working with Cloud PubSub.
package pubsub

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/emulators"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestEnsureNotEmulator_EnvNotVarSet_DoesNotPanic(t *testing.T) {
	unittest.SmallTest(t)
	oldValue := emulators.GetEmulatorHostEnvVar(emulators.PubSub)
	defer func() {
		os.Setenv(string(emulators.PubSub), oldValue)
	}()
	os.Setenv(string(emulators.PubSub), "")
	require.NotPanics(t, func() {
		EnsureNotEmulator()
	})
}

func TestEnsureNotEmulator_EnvVarSet_Panics(t *testing.T) {
	unittest.SmallTest(t)
	oldValue := emulators.GetEmulatorHostEnvVar(emulators.PubSub)
	defer func() {
		os.Setenv(string(emulators.PubSub), oldValue)
	}()
	os.Setenv(string(emulators.PubSub), "8080")
	require.Panics(t, func() {
		EnsureNotEmulator()
	})
}
