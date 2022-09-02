package ramdisk

import (
	"context"
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRamDisk(t *testing.T) {

	// Create a ram disk. Ensure that it creates a real directory.
	rd, cleanup, err := New(context.Background())
	require.NoError(t, err)
	st, err := os.Stat(rd)
	require.NoError(t, err)
	require.True(t, st.IsDir())
	// Remove the ram disk. Ensure that it no longer exists.
	cleanup()
	_, err = os.Stat(rd)
	require.True(t, os.IsNotExist(err))
}
