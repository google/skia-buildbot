package bt_gitstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/gitstore/shared_tests"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestGitStore(t *testing.T) {
	unittest.LargeTest(t)
	unittest.RequiresBigTableEmulator(t)

	conf := BTTestConfig()
	require.NoError(t, InitBT(conf))
	gs, err := New(context.Background(), conf, "fake.git")
	require.NoError(t, err)
	shared_tests.TestGitStore(t, gs)
}
