package git_common

import (
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skexec"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/testutils"
)

func TestVersion(t *testing.T) {
	testutils.SmallTest(t)
	major, minor, err := Version(skexec.NewExec())
	assert.NoError(t, err)
	sklog.Errorf("%d.%d", major, minor)
}
