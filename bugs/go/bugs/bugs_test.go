package bugs

import (
	"testing"

	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/testutils/unittest"
)

func TestBuganizer(t *testing.T) {
	unittest.SmallTest(t)

	require.Equal(t, 1, 1)
}
