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
		err := os.Setenv(string(emulators.PubSub), oldValue)
		require.NoError(t, err)
	}()
	err := os.Setenv(string(emulators.PubSub), "")
	require.NoError(t, err)
	require.NotPanics(t, func() {
		EnsureNotEmulator()
	})
}

func TestEnsureNotEmulator_EnvVarSet_Panics(t *testing.T) {
	unittest.SmallTest(t)
	oldValue := emulators.GetEmulatorHostEnvVar(emulators.PubSub)
	defer func() {
		err := os.Setenv(string(emulators.PubSub), oldValue)
		require.NoError(t, err)
	}()
	err := os.Setenv(string(emulators.PubSub), "8080")
	require.NoError(t, err)
	require.Panics(t, func() {
		EnsureNotEmulator()
	})
}
