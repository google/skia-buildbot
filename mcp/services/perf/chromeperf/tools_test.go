package chromeperf

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestMinimumViableSetOfRequiredFields_OK(t *testing.T) {
	tools := GetTools(nil)
	require.Equal(t, 3, len(tools))
}
