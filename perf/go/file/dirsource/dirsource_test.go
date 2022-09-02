package dirsource

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/file"
)

func TestStart_Success(t *testing.T) {
	ctx := context.Background()

	s, err := New("testdata")
	require.NoError(t, err)
	ch, err := s.Start(ctx)
	require.NoError(t, err)
	files := []file.File{}
	for f := range ch {
		files = append(files, f)
	}
	// Do some spot checking on the results.
	assert.Len(t, files, 2)
	// Both filenames end with .json.
	assert.True(t, strings.HasSuffix(files[0].Name, ".json"))
	// Both have their first byte as "{".
	buf := make([]byte, 1)
	_, err = files[0].Contents.Read(buf)
	require.NoError(t, err)
	assert.Equal(t, "{", string(buf))
}

func TestStart_SecondStartFails(t *testing.T) {
	ctx := context.Background()

	s, err := New("testdata")
	require.NoError(t, err)
	_, err = s.Start(ctx)
	require.NoError(t, err)
	_, err = s.Start(ctx)
	require.Error(t, err)
}
