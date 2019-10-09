package git_common

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestVersion(t *testing.T) {
	unittest.SmallTest(t)
	major, minor, err := Version(context.Background())
	require.NoError(t, err)
	sklog.Errorf("%d.%d", major, minor)
}
