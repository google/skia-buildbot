package testutils

import (
	"context"

	"go.skia.org/infra/go/emulators/gcp_emulator"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/sktest"
	"go.skia.org/infra/go/util"
)

// NewClientForTesting returns a Client and ensures that it will connect to the
// Firestore emulator. The Client's instance name will be randomized to ensure
// concurrent tests don't interfere with each other. It also returns a
// CleanupFunc that closes the Client.
func NewClientForTesting(ctx context.Context, t sktest.TestingT) (*firestore.Client, util.CleanupFunc) {
	gcp_emulator.RequireFirestore(t)
	return firestore.NewClientForTesting(ctx, t)
}
