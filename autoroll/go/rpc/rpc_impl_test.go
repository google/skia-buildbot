package rpc

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	manual_mocks "go.skia.org/infra/autoroll/go/manual/mocks"
	"go.skia.org/infra/autoroll/go/modes"
	modes_mocks "go.skia.org/infra/autoroll/go/modes/mocks"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/status"
	status_mocks "go.skia.org/infra/autoroll/go/status/mocks"
	strategy_mocks "go.skia.org/infra/autoroll/go/strategy/mocks"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	// Fake user emails.
	viewer = "viewer@google.com"
	editor = "editor@google.com"
	admin  = "admin@google.com"
)

var (
	// Allow fake users.
	viewers = allowed.NewAllowedFromList([]string{viewer})
	editors = allowed.NewAllowedFromList([]string{editor})
	admins  = allowed.NewAllowedFromList([]string{admin})
)

func makeRoller(ctx context.Context, t *testing.T, name string) *AutoRoller {
	cfg := &roller.AutoRollerConfig{
		ChildDisplayName:  name + "_child",
		ParentDisplayName: name + "_parent",
		RollerName:        name,
	}
	statusDB := &status_mocks.DB{}
	statusDB.On("Get", ctx, name).Return(&status.AutoRollStatus{
		// TODO
	}, nil)
	statusCache, err := status.NewCache(ctx, statusDB, name)
	require.NoError(t, err)
	return &AutoRoller{
		Cfg:      cfg,
		Mode:     &modes_mocks.ModeHistory{},
		Status:   statusCache,
		Strategy: &strategy_mocks.StrategyHistory{},
	}
}

func setup(t *testing.T) (context.Context, map[string]*AutoRoller, *autoRollServerImpl) {
	ctx := context.Background()
	r1 := makeRoller(ctx, t, "roller1")
	r2 := makeRoller(ctx, t, "roller2")
	rollers := map[string]*AutoRoller{
		r1.Cfg.RollerName: r1,
		r2.Cfg.RollerName: r2,
	}
	mdb := &manual_mocks.DB{}
	srv := newAutoRollServerImpl(rollers, mdb, viewers, editors, admins)
	return ctx, rollers, srv
}

func TestGetRollers(t *testing.T) {
	unittest.SmallTest(t)

	ctx, rollers, srv := setup(t)

	// Mocks.
	for _, roller := range rollers {
		roller.Mode.(*modes_mocks.ModeHistory).On("CurrentMode").Return(&modes.ModeChange{
			Mode: modes.ModeRunning,
		})
	}
	req := &GetRollersRequest{}

	// Check authorization.
	mockUser := ""
	srv.MockGetUserForTesting(func(ctx context.Context) string {
		return mockUser
	})
	res, err := srv.GetRollers(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized viewer")
	mockUser = "no-access@google.com"
	res, err = srv.GetRollers(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"no-access@google.com\" is not an authorized viewer")

	// Check results.
	mockUser = viewer
	res, err = srv.GetRollers(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res)
	expectRollers := make([]*AutoRollMiniStatus, 0, len(rollers))
	for _, roller := range rollers {
		expectRollers = append(expectRollers, &AutoRollMiniStatus{
			ChildName:  roller.Cfg.ChildDisplayName,
			Mode:       roller.Mode.CurrentMode().Mode,
			ParentName: roller.Cfg.ParentDisplayName,
			Roller:     roller.Cfg.RollerName,
		})
	}
	assertdeep.Equal(t, &AutoRollMiniStatuses{
		Statuses: expectRollers,
	}, res)
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
