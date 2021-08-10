// Package hiddenstore handles storing and retrieving the 'hidden' status of any URL for a given search hashtag.
package hiddenstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/firestore/testutils"
	"go.skia.org/infra/go/testutils/unittest"
)

func TestHiddenStore_SetHidden(t *testing.T) {
	unittest.LargeTest(t)
	client, cleanup := testutils.NewClientForTesting(context.Background(), t)
	defer cleanup()

	h := HiddenStore{
		client:           client,
		hiddenCollection: client.Collection("hashtag"),
	}

	// Add a hidden.
	ctx := context.Background()
	err := h.SetHidden(ctx, "foo", "https://example.org", true)
	assert.NoError(t, err)
	list := h.GetHidden(ctx, "foo")
	assert.ElementsMatch(t, []string{"https://example.org"}, list)

	// Add the same thing a second time, which should be a noop.
	err = h.SetHidden(ctx, "foo", "https://example.org", true)
	assert.NoError(t, err)
	list = h.GetHidden(ctx, "foo")
	assert.ElementsMatch(t, []string{"https://example.org"}, list)

	// Add a second hidden.
	err = h.SetHidden(ctx, "foo", "http://example.com", true)
	assert.NoError(t, err)
	list = h.GetHidden(ctx, "foo")
	assert.ElementsMatch(t, []string{"https://example.org", "http://example.com"}, list)

	// Remove one.
	err = h.SetHidden(ctx, "foo", "https://example.org", false)
	assert.NoError(t, err)
	list = h.GetHidden(ctx, "foo")
	assert.ElementsMatch(t, []string{"http://example.com"}, list)

	// Remove something that doesn't exist.
	err = h.SetHidden(ctx, "bar", "https://example.org", false)
	assert.NoError(t, err)

}
