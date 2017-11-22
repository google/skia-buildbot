package git_common

import (
	"context"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
)

func TestVersion(t *testing.T) {
	testutils.SmallTest(t)
	major, minor, err := Version(context.Background())
	assert.NoError(t, err)
	sklog.Errorf("%d.%d", major, minor)
}
