package types

import (
	"context"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/attest/go/types/mocks"
	cache_mocks "go.skia.org/infra/go/cache/mock"
	"go.skia.org/infra/go/testutils"
)

func TestValidateAttestor(t *testing.T) {
	valid := func(attestor string) {
		require.NoError(t, ValidateAttestor(attestor))
	}
	notValid := func(attestor string) {
		require.Error(t, ValidateAttestor(attestor))
	}

	valid("projects/louhi-prod-attest/attestors/built_by_louhi_prod")
	notValid("built_by_louhi_prod")
	notValid("rojects/louhi-prod-attest/attestors/built_by_louhi_prod")
	notValid("projects/louhi-prod-attest/ttestors/built_by_louhi_prod")
}

func TestValidateImageID(t *testing.T) {
	valid := func(imageID string) {
		require.NoError(t, ValidateImageID(imageID))
	}
	notValid := func(imageID string) {
		err := ValidateImageID(imageID)
		require.Error(t, err)
		require.True(t, IsErrBadImageFormat(err))
	}

	valid("gcr.io/skia-public/task-scheduler-be@sha256:d5062719ba4240c2d5b5beb31882b8beba584a6e218464365e7b117404ca8992")
	notValid("some-bogus-image")
	notValid("gcr.io/skia-public/task-scheduler-be@sha256:d5062719ba4240c2d5b5beb31882b8beba584a6e218464365e7b117404ca899")
	notValid("gcr.io/skia-public/task-scheduler-be@sha256:d5062719ba4240c2d5b5beb31882b8beba584a6e218464365e7b117404ca89923")
	notValid("gcr.io/skia-public/task-scheduler-be:some-tag")
}

func TestClientServer_Verified(t *testing.T) {
	mockClient := &mocks.Client{}
	server := NewServer(mockClient)
	ts := httptest.NewServer(server)
	defer ts.Close()
	client := NewHttpClient(ts.URL, ts.Client())

	// Image was verified.
	const verifiedImageID = "gcr.io/skia-public/verified@sha256:d5062719ba4240c2d5b5beb31882b8beba584a6e218464365e7b117404ca8992"
	mockClient.On("Verify", testutils.AnyContext, verifiedImageID).Return(true, nil).Once()
	verified, err := client.Verify(t.Context(), verifiedImageID)
	require.NoError(t, err)
	require.True(t, verified)
	mockClient.AssertExpectations(t)
}

func TestClientServer_NotVerified(t *testing.T) {
	mockClient := &mocks.Client{}
	server := NewServer(mockClient)
	ts := httptest.NewServer(server)
	defer ts.Close()
	client := NewHttpClient(ts.URL, ts.Client())

	// Image not verified.
	const unVerifiedImageID = "gcr.io/skia-public/unverified@sha256:d5062719ba4240c2d5b5beb31882b8beba584a6e218464365e7b117404ca8992"
	mockClient.On("Verify", testutils.AnyContext, unVerifiedImageID).Return(false, nil).Once()
	verified, err := client.Verify(t.Context(), unVerifiedImageID)
	require.NoError(t, err)
	require.False(t, verified)
	mockClient.AssertExpectations(t)
}

func TestClientServer_BadImageFormat(t *testing.T) {
	mockClient := &mocks.Client{}
	server := NewServer(mockClient)
	ts := httptest.NewServer(server)
	defer ts.Close()
	client := NewHttpClient(ts.URL, ts.Client())

	// Bad image format. This doesn't actually perform a round trip, since we
	// perform the check on the client side.
	verified, err := client.Verify(t.Context(), "bogus")
	require.True(t, IsErrBadImageFormat(err))
	require.False(t, verified)
}

func TestClientWithCache_NotCached_Verified(t *testing.T) {
	ctx := context.Background()
	mockClient := &mocks.Client{}
	mockCache := &cache_mocks.Cache{}
	client := WithCache(mockClient, mockCache)

	const imageID = "fake-image-id"
	mockCache.On("GetValue", testutils.AnyContext, imageID).Return("", nil).Once()
	mockClient.On("Verify", testutils.AnyContext, imageID).Return(true, nil).Once()
	mockCache.On("SetValue", testutils.AnyContext, imageID, cachedValueTrue).Return(nil).Once()
	verified, err := client.Verify(ctx, imageID)
	require.NoError(t, err)
	require.True(t, verified)
	mockCache.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

func TestClientWithCache_Cached_Verified(t *testing.T) {
	ctx := context.Background()
	mockClient := &mocks.Client{}
	mockCache := &cache_mocks.Cache{}
	client := WithCache(mockClient, mockCache)

	const imageID = "fake-image-id"
	mockCache.On("GetValue", testutils.AnyContext, imageID).Return(cachedValueTrue, nil).Once()
	verified, err := client.Verify(ctx, imageID)
	require.NoError(t, err)
	require.True(t, verified)
	mockCache.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

func TestClientWithCache_NotCached_NotVerified(t *testing.T) {
	ctx := context.Background()
	mockClient := &mocks.Client{}
	mockCache := &cache_mocks.Cache{}
	client := WithCache(mockClient, mockCache)

	const imageID = "fake-image-id"
	mockCache.On("GetValue", testutils.AnyContext, imageID).Return("", nil).Once()
	mockClient.On("Verify", testutils.AnyContext, imageID).Return(false, nil).Once()
	mockCache.On("SetValue", testutils.AnyContext, imageID, cachedValueFalse).Return(nil).Once()
	verified, err := client.Verify(ctx, imageID)
	require.NoError(t, err)
	require.False(t, verified)
	mockCache.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}

func TestClientWithCache_Cached_NotVerified(t *testing.T) {
	ctx := context.Background()
	mockClient := &mocks.Client{}
	mockCache := &cache_mocks.Cache{}
	client := WithCache(mockClient, mockCache)

	const imageID = "fake-image-id"
	mockCache.On("GetValue", testutils.AnyContext, imageID).Return(cachedValueFalse, nil).Once()
	verified, err := client.Verify(ctx, imageID)
	require.NoError(t, err)
	require.False(t, verified)
	mockCache.AssertExpectations(t)
	mockClient.AssertExpectations(t)
}
