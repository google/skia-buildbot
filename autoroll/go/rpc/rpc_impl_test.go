package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	manual_mocks "go.skia.org/infra/autoroll/go/manual/mocks"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/testutils/unittest"
)

func setup(t *testing.T) {

}

func TestGetRollers(t *testing.T) {
	unittest.SmallTest(t)

	// Setup.
	ctx := context.Background()
	rollers := map[string]*AutoRoller{
		"roller1": {},
		"roller2": {},
	}
	mdb := &manual_mocks.DB{}
	viewers := allowed.NewAllowedFromList([]string{"viewer@google.com"})
	editors := allowed.NewAllowedFromList([]string{"editor@google.com"})
	admins := allowed.NewAllowedFromList([]string{"admin@google.com"})
	srv := newAutoRollServerImpl(rollers, mdb, viewers, editors, admins)
	mockUser := ""
	srv.MockGetUserForTesting(func(ctx context.Context) string {
		return mockUser
	})

	// Check authorization.
	_, err := srv.GetRollers(ctx, nil)
	require.Equal(t, err, nil) // TODO

	// Check results.
}

func TestGetMiniStatus(t *testing.T) {
	unittest.SmallTest(t)

	// Check authorization.

	// Check error for unknown roller.

	// Check results.
}

func TestGetStatus(t *testing.T) {
	unittest.SmallTest(t)

	// Check authorization.

	// Check error for unknown roller.

	// Check results.
}

func TestSetMode(t *testing.T) {
	unittest.SmallTest(t)

	// Check authorization.

	// Check error for unknown roller.

	// Check results.
}

func TestSetStrategy(t *testing.T) {
	unittest.SmallTest(t)

	// Check authorization.

	// Check error for unknown roller.

	// Check results.
}

func TestCreateManualRoll(t *testing.T) {
	unittest.SmallTest(t)

	// Check authorization.

	// Check error for unknown roller.

	// Check results.
}

func TestUnthrottle(t *testing.T) {
	unittest.SmallTest(t)

	// Check authorization.

	// Check error for unknown roller.

	// Check results.
}

// TODO(borenet): Explicitly check type conversions.
