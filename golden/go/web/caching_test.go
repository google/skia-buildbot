package web

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/cache/local"
	"go.skia.org/infra/golden/go/web/frontend"
)

func TestWebCacheManager_GetBaseline_Success(t *testing.T) {
	ctx := context.Background()
	lc, err := local.New(100)
	require.NoError(t, err)

	wcm := NewCacheManager(lc)

	const crs = "github"
	const clID = "12345"
	expected := &frontend.BaselineV2Response{
		ChangelistID:     clID,
		CodeReviewSystem: crs,
	}
	b, err := json.Marshal(expected)
	require.NoError(t, err)

	err = lc.SetValue(ctx, "baseline_github_12345", string(b))
	require.NoError(t, err)

	actual, err := wcm.GetBaseline(ctx, crs, clID)
	require.NoError(t, err)
	assert.Equal(t, expected, actual)
}

func TestWebCacheManager_GetBaseline_NotFound(t *testing.T) {
	ctx := context.Background()
	lc, err := local.New(100)
	require.NoError(t, err)

	wcm := NewCacheManager(lc)

	const crs = "github"
	const clID = "12345"

	actual, err := wcm.GetBaseline(ctx, crs, clID)
	require.NoError(t, err)
	assert.Nil(t, actual)
}

func TestWebCacheManager_SetBaseline_Success(t *testing.T) {
	ctx := context.Background()
	lc, err := local.New(100)
	require.NoError(t, err)

	wcm := NewCacheManager(lc)

	const crs = "github"
	const clID = "12345"
	expected := frontend.BaselineV2Response{
		ChangelistID:     clID,
		CodeReviewSystem: crs,
	}

	err = wcm.SetBaseline(ctx, crs, clID, expected, 2*time.Second)
	require.NoError(t, err)

	actualJSON, err := lc.GetValue(ctx, "baseline_github_12345")
	require.NoError(t, err)

	var actual frontend.BaselineV2Response
	err = json.Unmarshal([]byte(actualJSON), &actual)
	require.NoError(t, err)

	assert.Equal(t, expected, actual)
}

func TestWebCacheManager_GetAndSetBaseline(t *testing.T) {
	ctx := context.Background()
	lc, err := local.New(100)
	require.NoError(t, err)

	wcm := NewCacheManager(lc)

	// Get baseline when cache is empty.
	const crs = "gerrit"
	const clID = "67890"
	actual, err := wcm.GetBaseline(ctx, crs, clID)
	require.NoError(t, err)
	assert.Nil(t, actual)

	// Set baseline.
	expected := frontend.BaselineV2Response{
		ChangelistID:     clID,
		CodeReviewSystem: crs,
	}
	err = wcm.SetBaseline(ctx, crs, clID, expected, 2*time.Second)
	require.NoError(t, err)

	// Get baseline again.
	actual, err = wcm.GetBaseline(ctx, crs, clID)
	require.NoError(t, err)
	assert.Equal(t, &expected, actual)
}
