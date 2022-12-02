package bt_gitstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/emulators/gcp_emulator"
	"go.skia.org/infra/go/gitstore/shared_tests"
)

func TestGitStore(t *testing.T) {
	gcp_emulator.RequireBigTable(t)

	conf := BTTestConfig()
	require.NoError(t, InitBT(conf))
	gs, err := New(context.Background(), conf, "fake.git")
	require.NoError(t, err)
	shared_tests.TestGitStore(t, gs)
}
