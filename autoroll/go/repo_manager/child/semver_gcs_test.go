package child

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestSemVerGCSShortRev(t *testing.T) {
	require.Equal(t, "123", semVerShortRev("\\d+", "abc123def"))
	require.Equal(t, "123", semVerShortRev("\\d+", "abc123def456"))
	require.Equal(t, "abc123def456", semVerShortRev("[a-z]+\\d+[a-z]+\\d+", "abc123def456"))
	require.Equal(t, "456", semVerShortRev("[a-z]+\\d+[a-z]+(\\d+)", "abc123def456"))
	require.Equal(t, "123", semVerShortRev("[a-z]+(\\d+)[a-z]+(\\d+)", "abc123def456"))
	require.Equal(t, "abcdef0123456789abcdef0123456789abcdef01", semVerShortRev(".{7}.{33}", "abcdef0123456789abcdef0123456789abcdef01"))
	require.Equal(t, "abcdef0", semVerShortRev("(.{7}).{33}", "abcdef0123456789abcdef0123456789abcdef01"))
}
