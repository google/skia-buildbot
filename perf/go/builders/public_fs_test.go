package builders

import (
	"context"
	"fmt"
	"io/fs"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockFS struct {
	openedName string
	openErr    error

	prefixSearchPrefix string
	prefixSearchResult string
	prefixSearchErr    error
}

func (m *mockFS) Open(name string) (fs.File, error) {
	m.openedName = name
	if m.prefixSearchResult != "" && name == m.prefixSearchResult {
		return nil, nil // Simulate successful open of fallback file
	}
	return nil, m.openErr
}

func (m *mockFS) FindFileByPrefix(ctx context.Context, namePrefix string) (string, error) {
	m.prefixSearchPrefix = namePrefix
	return m.prefixSearchResult, m.prefixSearchErr
}

func TestPublicFS_Success(t *testing.T) {
	mock := &mockFS{}
	fs := &publicFS{
		FS:          mock,
		overrideGCS: "chrome-perf-public",
	}

	_, _ = fs.Open("gs://chrome-perf-non-public/ingest/2026/file.json")
	assert.Equal(t, "gs://chrome-perf-public/ingest/2026/file.json", mock.openedName)

	_, _ = fs.Open("gs://other-bucket/ingest/2026/file.json")
	assert.Equal(t, "gs://chrome-perf-public/ingest/2026/file.json", mock.openedName)

	_, _ = fs.Open("some/local/file.json")
	assert.Equal(t, "some/local/file.json", mock.openedName)
}

func TestPublicFS_Fallback_Success(t *testing.T) {
	mock := &mockFS{
		openErr:            fmt.Errorf("file not found"),
		prefixSearchResult: "gs://chrome-perf-public/ingest/2026/05/06/ChromiumPerf/mac-m2-pro-perf/18844/rendering.desktop/skia_results_rendering.desktop_mac-m2-pro-perf_18844_2026_05_06_T20_00_29-UTC.json",
	}
	fs := &publicFS{
		FS:          mock,
		overrideGCS: "chrome-perf-public",
	}

	_, err := fs.Open("gs://chrome-perf-non-public/ingest/2026/05/06/ChromiumPerf/mac-m2-pro-perf/18844/rendering.desktop/skia_results_rendering.desktop_mac-m2-pro-perf_18844_2026_05_06_T20_00_30-UTC.json")

	assert.Equal(t, "gs://chrome-perf-public/ingest/2026/05/06/ChromiumPerf/mac-m2-pro-perf/18844/rendering.desktop/skia_results_rendering.desktop_mac-m2-pro-perf_18844", mock.prefixSearchPrefix)
	assert.Equal(t, "gs://chrome-perf-public/ingest/2026/05/06/ChromiumPerf/mac-m2-pro-perf/18844/rendering.desktop/skia_results_rendering.desktop_mac-m2-pro-perf_18844_2026_05_06_T20_00_29-UTC.json", mock.openedName)
	assert.NoError(t, err)
}

func TestPublicFS_Fallback_FailureReturnsOriginalError(t *testing.T) {
	origErr := fmt.Errorf("original not found")
	mock := &mockFS{
		openErr:         origErr,
		prefixSearchErr: fmt.Errorf("prefix not found"),
	}
	fs := &publicFS{
		FS:          mock,
		overrideGCS: "chrome-perf-public",
	}

	_, err := fs.Open("gs://chrome-perf-non-public/ingest/2026/05/06/ChromiumPerf/mac-m2-pro-perf/18844/rendering.desktop/skia_results_rendering.desktop_mac-m2-pro-perf_18844_2026_05_06_T20_00_30-UTC.json")

	assert.Equal(t, origErr, err)
}
