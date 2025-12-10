package pgadapter_jar

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFindPGAdapterJar(t *testing.T) {
	path := FindPGAdapterJar()
	fileInfo, err := os.Stat(path)
	require.NoError(t, err)
	require.False(t, fileInfo.IsDir())
	require.NotZero(t, fileInfo.Size())
}
