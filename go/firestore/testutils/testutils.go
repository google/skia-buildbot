package firestore_testutils

import (
	"time"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/testutils"
)

// InitFirestore is a common utility function used in tests. It sets up the
// Client to connect to the Firestore emulator and returns a CleanupFunc that
// removes all documents created with the returned Client, and then Closes the
// client.
func InitFirestore(t assert.TestingT) (*firestore.Client, testutils.CleanupFunc) {
	c, err := firestore.NewClientForTesting()
	assert.NoError(t, err)
	return func() {
		assert.NoError(t, c.RecursiveDelete(c.ParentDoc, 5, 30*time.Second))
		assert.NoError(t, c.Close())
	}
}
