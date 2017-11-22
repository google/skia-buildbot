package trybotstore

import (
	"testing"

	assert "github.com/stretchr/testify/require"

	"go.skia.org/infra/go/common"
)

func TestCloudTrybotStore(t *testing.T) {
	store, err := NewCloudTrybotStore(common.PROJECT_ID, "gold-testing-tarock", "service-account.json")
	assert.NoError(t, err)

	testTrybotStore(t, store)
}

func TestMemTrybotStore(t *testing.T) {
	store := NewMemTrybotStore()
	testTrybotStore(t, store)
}

func testTrybotStore(t *testing.T, store TrybotStore) {
	// Add trybot builds to the store.

}
