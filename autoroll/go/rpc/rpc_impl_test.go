package rpc

import (
	"context"
	fmt "fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config/db"
	config_db_mocks "go.skia.org/infra/autoroll/go/config/db/mocks"
	"go.skia.org/infra/autoroll/go/manual"
	manual_mocks "go.skia.org/infra/autoroll/go/manual/mocks"
	"go.skia.org/infra/autoroll/go/modes"
	modes_mocks "go.skia.org/infra/autoroll/go/modes/mocks"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	rolls_mocks "go.skia.org/infra/autoroll/go/recent_rolls/mocks"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/roller_cleanup"
	cleanup_mocks "go.skia.org/infra/autoroll/go/roller_cleanup/mocks"
	"go.skia.org/infra/autoroll/go/status"
	status_mocks "go.skia.org/infra/autoroll/go/status/mocks"
	"go.skia.org/infra/autoroll/go/strategy"
	strategy_mocks "go.skia.org/infra/autoroll/go/strategy/mocks"
	unthrottle_mocks "go.skia.org/infra/autoroll/go/unthrottle/mocks"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/alogin/mocks"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/roles"
	"go.skia.org/infra/go/testutils"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Fake user emails.
	noAccess = "no-access@google.com"
	viewer   = "viewer@google.com"
	editor   = "editor@google.com"
	admin    = "admin@google.com"
)

var (
	notLoggedInStatus = alogin.Status{
		EMail: alogin.NotLoggedIn,
		Roles: roles.Roles{},
	}

	viewerStatus = alogin.Status{
		EMail: alogin.EMail(viewer),
		Roles: roles.Roles{roles.Viewer},
	}

	editorStatus = alogin.Status{
		EMail: alogin.EMail(editor),
		Roles: roles.Roles{roles.Editor},
	}

	// Current at time of writing.
	currentTime = time.Unix(1598467386, 0).UTC()
)

func makeFakeModeChange(cfg *config.Config) *modes.ModeChange {
	return &modes.ModeChange{
		Message: "dry run!",
		// We can't use the first enum value, or assertdeep.Copy will fail.
		Mode:   modes.ModeDryRun,
		Roller: cfg.RollerName,
		Time:   timeNowFunc(),
		User:   "me@google.com",
	}
}

func makeFakeStrategyChange(cfg *config.Config) *strategy.StrategyChange {
	return &strategy.StrategyChange{
		Message: "set strategy",
		// We can't use the first enum value, or assertdeep.Copy will fail.
		Strategy: strategy.ROLL_STRATEGY_N_BATCH,
		Roller:   cfg.RollerName,
		Time:     timeNowFunc(),
		User:     "you@google.com",
	}
}

func makeFakeManualRollRequest(cfg *config.Config) *manual.ManualRollRequest {
	return &manual.ManualRollRequest{
		Id:                "manual123",
		DbModified:        timeNowFunc(),
		Requester:         "me@google.com",
		Result:            manual.RESULT_UNKNOWN,
		ResultDetails:     "no results yet",
		Revision:          "fake-rev",
		RollerName:        cfg.RollerName,
		Status:            manual.STATUS_STARTED,
		Timestamp:         timeNowFunc(),
		Url:               "https://fake-manual-roll",
		DryRun:            true,
		NoEmail:           true,
		NoResolveRevision: true,
	}
}

func makeFakeRoll() *autoroll.AutoRollIssue {
	return &autoroll.AutoRollIssue{
		Closed:         false,
		IsDryRun:       false,
		DryRunFinished: false,
		DryRunSuccess:  false,
		CqFinished:     false,
		CqSuccess:      false,
		Issue:          12345,
		Result:         autoroll.ROLL_RESULT_DRY_RUN_IN_PROGRESS,
		RollingFrom:    "abc123",
		RollingTo:      "def456",
		Subject:        "Roll dep from abc123 to def456",
		TryResults: []*autoroll.TryResult{
			{
				Builder:  "test-bot",
				Category: "cq",
				Created:  timeNowFunc(),
				Result:   autoroll.TRYBOT_RESULT_SUCCESS,
				Status:   autoroll.TRYBOT_STATUS_COMPLETED,
				Url:      "https://fake-try.result",
			},
			{
				Builder:  "perf-bot",
				Category: "cq",
				Created:  timeNowFunc(),
				Result:   autoroll.TRYBOT_RESULT_CANCELED,
				Status:   autoroll.TRYBOT_STATUS_STARTED,
				Url:      "https://fake-try.result",
			},
		},
	}
}

func makeFakeStatus(cfg *config.Config) *status.AutoRollStatus {
	current := makeFakeRoll()
	last := makeFakeRoll()
	return &status.AutoRollStatus{
		AutoRollMiniStatus: status.AutoRollMiniStatus{
			Mode:                        modes.ModeRunning,
			CurrentRollRev:              "def456",
			LastRollRev:                 "abc123",
			NumFailedRolls:              1,
			NumNotRolledCommits:         2,
			Timestamp:                   currentTime,
			LastSuccessfulRollTimestamp: currentTime,
		},
		Status:         "rolling",
		ChildHead:      "def456",
		ChildName:      cfg.ChildDisplayName,
		FullHistoryUrl: "http://fake",
		IssueUrlBase:   "http://fake2",
		NotRolledRevisions: []*revision.Revision{
			{
				Id:          "999abc",
				Description: "Commit 999abc",
				Display:     "9",
				Timestamp:   timeNowFunc(),
				URL:         "https://rev/999abc",
			},
			{
				Id:          "def456",
				Description: "Commit def456",
				Display:     "d",
				Timestamp:   timeNowFunc(),
				URL:         "https://rev/def456",
			},
		},
		ParentName:      cfg.ParentDisplayName,
		CurrentRoll:     current,
		LastRoll:        last,
		Recent:          []*autoroll.AutoRollIssue{current, last},
		ValidModes:      modes.ValidModes,
		ValidStrategies: cfg.ValidStrategies(),
		Error:           "no error",
		ThrottledUntil:  timeNowFunc().Unix(),
	}
}

func makeRoller(ctx context.Context, t *testing.T, name string, mdb *manual_mocks.DB, cdb *cleanup_mocks.DB) *AutoRoller {
	cfg := &config.Config{
		ChildBugLink:      "https://child-bug.com",
		ChildDisplayName:  name + "_child",
		ParentBugLink:     "https://parent-bug.com",
		ParentDisplayName: name + "_parent",
		ParentWaterfall:   "https://parent",
		RollerName:        name,
		RepoManager: &config.Config_ParentChildRepoManager{
			ParentChildRepoManager: &config.ParentChildRepoManagerConfig{
				Child: &config.ParentChildRepoManagerConfig_GitilesChild{
					GitilesChild: &config.GitilesChildConfig{
						Gitiles: &config.GitilesConfig{
							Branch:  git.MainBranch,
							RepoUrl: "https://fake.child",
						},
					},
				},
				Parent: &config.ParentChildRepoManagerConfig_GitilesParent{
					GitilesParent: &config.GitilesParentConfig{
						Gitiles: &config.GitilesConfig{
							Branch:  git.MainBranch,
							RepoUrl: "https://fake.parent",
						},
						Dep: &config.DependencyConfig{
							Primary: &config.VersionFileConfig{
								Id: "https://fake.child",
							},
						},
						Gerrit: &config.GerritConfig{},
					},
				},
			},
		},
		SupportsManualRolls: true,
		TimeWindow:          "24h",
	}
	currentStatus := makeFakeStatus(cfg)
	statusDB := &status_mocks.DB{}
	statusDB.On("Get", ctx, name).Return(currentStatus, nil)
	statusCache, err := status.NewCache(ctx, statusDB, name)
	require.NoError(t, err)

	modeHistory := &modes_mocks.ModeHistory{}
	modeHistory.On("CurrentMode").Return(makeFakeModeChange(cfg))
	strategyHistory := &strategy_mocks.StrategyHistory{}
	strategyHistory.On("CurrentStrategy").Return(makeFakeStrategyChange(cfg))

	manualReq := makeFakeManualRollRequest(cfg)
	mdb.On("GetRecent", cfg.RollerName, recent_rolls.RecentRollsLength).Return([]*manual.ManualRollRequest{manualReq}, nil)

	cdb.On("History", testutils.AnyContext, name, 1).Return([]*roller_cleanup.CleanupRequest{{
		RollerID:      name,
		NeedsCleanup:  true,
		User:          "editor@google.com",
		Timestamp:     currentTime,
		Justification: "needs cleanup",
	}}, nil)

	return &AutoRoller{
		Cfg:      cfg,
		Mode:     modeHistory,
		Status:   statusCache,
		Strategy: strategyHistory,
	}
}

func setup(t *testing.T) (context.Context, map[string]*AutoRoller, *AutoRollServer) {
	timeNowFunc = func() time.Time {
		return currentTime
	}
	ctx := context.Background()
	cdb := &config_db_mocks.DB{}
	cleanupDB := &cleanup_mocks.DB{}
	mdb := &manual_mocks.DB{}
	sdb := &status_mocks.DB{}
	rdb := &rolls_mocks.DB{}
	r1 := makeRoller(ctx, t, "roller1", mdb, cleanupDB)
	r2 := makeRoller(ctx, t, "roller2", mdb, cleanupDB)
	rollers := map[string]*AutoRoller{
		r1.Cfg.RollerName: r1,
		r2.Cfg.RollerName: r2,
	}
	loadRollersFunc = func(context.Context, status.DB, db.DB) (map[string]*AutoRoller, context.CancelFunc, error) {
		return rollers, func() {}, nil
	}
	plogin := mocks.NewLogin(t)
	srv, err := NewAutoRollServer(ctx, sdb, cdb, rdb, mdb, cleanupDB, &unthrottle_mocks.Throttle{}, time.Duration(0), plogin)
	require.NoError(t, err)
	return ctx, rollers, srv
}

func TestGetRollers(t *testing.T) {

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	req := &GetRollersRequest{}

	ctx = alogin.FakeStatus(ctx, &notLoggedInStatus)
	res, err := srv.GetRollers(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res)
	expectRollers := make([]*AutoRollMiniStatus, 0, len(rollers))
	for _, roller := range rollers {
		status := makeFakeStatus(roller.Cfg)
		ms, err := convertMiniStatus(&status.AutoRollMiniStatus, roller.Cfg.RollerName, roller.Mode.CurrentMode().Mode, roller.Cfg.ChildDisplayName, roller.Cfg.ParentDisplayName)
		require.NoError(t, err)
		expectRollers = append(expectRollers, ms)
	}
	sort.Sort(autoRollMiniStatusSlice(expectRollers))
	assertdeep.Equal(t, &GetRollersResponse{
		Rollers: expectRollers,
	}, res)
}

func TestGetRolls(t *testing.T) {
	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &GetRollsRequest{
		RollerId: "this roller doesn't exist",
	}

	// Check error for unknown roller.
	ctx = alogin.FakeStatus(ctx, &notLoggedInStatus)
	res, err := srv.GetRolls(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check results.
	req.RollerId = roller.Cfg.RollerName
	ctx = alogin.FakeStatus(ctx, &editorStatus)
	srv.rollsDB.(*rolls_mocks.DB).On("GetRolls", testutils.AnyContext, req.RollerId, "").Return([]*autoroll.AutoRollIssue{}, "", nil)
	res, err = srv.GetRolls(ctx, req)
	require.NoError(t, err)
	require.NotNil(t, res)
}

func TestGetMiniStatus(t *testing.T) {

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &GetMiniStatusRequest{
		RollerId: "this roller doesn't exist",
	}

	// Check error for unknown roller.
	ctx = alogin.FakeStatus(ctx, &notLoggedInStatus)
	res, err := srv.GetMiniStatus(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check results.
	req.RollerId = roller.Cfg.RollerName
	res, err = srv.GetMiniStatus(ctx, req)
	require.NoError(t, err)
	mode, err := convertMode(roller.Mode.CurrentMode().Mode)
	require.NoError(t, err)
	assertdeep.Equal(t, &GetMiniStatusResponse{
		Status: &AutoRollMiniStatus{
			ChildName:                   roller.Cfg.ChildDisplayName,
			Mode:                        mode,
			ParentName:                  roller.Cfg.ParentDisplayName,
			RollerId:                    roller.Cfg.RollerName,
			CurrentRollRev:              "def456",
			LastRollRev:                 "abc123",
			NumFailed:                   1,
			NumBehind:                   2,
			Timestamp:                   timestamppb.New(currentTime),
			LastSuccessfulRollTimestamp: timestamppb.New(currentTime),
		},
	}, res)
}

func TestGetStatus(t *testing.T) {

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &GetStatusRequest{
		RollerId: "this roller doesn't exist",
	}

	// Check error for unknown roller.
	ctx = alogin.FakeStatus(ctx, &notLoggedInStatus)
	res, err := srv.GetStatus(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check results.
	req.RollerId = roller.Cfg.RollerName
	res, err = srv.GetStatus(ctx, req)
	require.NoError(t, err)
	st := makeFakeStatus(roller.Cfg)
	manualReqs, err := srv.manualRollDB.GetRecent(roller.Cfg.RollerName, recent_rolls.RecentRollsLength)
	require.NoError(t, err)
	cleanupHistory, err := srv.cleanupDB.History(ctx, roller.Cfg.RollerName, 1)
	require.NoError(t, err)
	expect, err := convertStatus(st, roller.Cfg, roller.Mode.CurrentMode(), roller.Strategy.CurrentStrategy(), manualReqs, cleanupHistory)
	require.NoError(t, err)
	assertdeep.Equal(t, &GetStatusResponse{
		Status: expect,
	}, res)
}

func TestSetMode(t *testing.T) {

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &SetModeRequest{
		Message:  "new mode",
		Mode:     Mode_DRY_RUN,
		RollerId: "this roller doesn't exist",
	}

	// Check authorization.
	ctx = alogin.FakeStatus(ctx, &notLoggedInStatus)
	res, err := srv.SetMode(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")
	ctx = alogin.FakeStatus(ctx, &viewerStatus)
	res, err = srv.SetMode(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check error for unknown roller.
	ctx = alogin.FakeStatus(ctx, &editorStatus)
	res, err = srv.SetMode(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check error for invalid mode for this roller.
	req.RollerId = roller.Cfg.RollerName
	roller.Cfg.ValidModes = []config.Mode{config.Mode_RUNNING, config.Mode_STOPPED, config.Mode_OFFLINE}
	res, err = srv.SetMode(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error invalid_argument: mode requested mode is not allowed for this roller; valid modes: [RUNNING STOPPED OFFLINE]")

	// Ensure no error when ValidModes is not set. Check for a valid result.
	roller.Cfg.ValidModes = nil
	roller.Mode.(*modes_mocks.ModeHistory).On("Add", ctx, modes.ModeDryRun, editor, req.Message).Return(nil)
	res, err = srv.SetMode(ctx, req)
	require.NoError(t, err)
	st := makeFakeStatus(roller.Cfg)
	manualReqs, err := srv.manualRollDB.GetRecent(roller.Cfg.RollerName, recent_rolls.RecentRollsLength)
	require.NoError(t, err)
	cleanupHistory, err := srv.cleanupDB.History(ctx, roller.Cfg.RollerName, 1)
	require.NoError(t, err)
	expect, err := convertStatus(st, roller.Cfg, roller.Mode.CurrentMode(), roller.Strategy.CurrentStrategy(), manualReqs, cleanupHistory)
	require.NoError(t, err)
	assertdeep.Equal(t, &SetModeResponse{
		Status: expect,
	}, res)

	// Ensure no error when ValidModes is set.
	roller.Cfg.ValidModes = []config.Mode{config.Mode_RUNNING, config.Mode_DRY_RUN, config.Mode_STOPPED, config.Mode_OFFLINE}
	roller.Mode.(*modes_mocks.ModeHistory).On("Add", ctx, modes.ModeDryRun, editor, req.Message).Return(nil)
	_, err = srv.SetMode(ctx, req)
	require.NoError(t, err)
}

func TestSetStrategy(t *testing.T) {

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &SetStrategyRequest{
		Message:  "new strategy",
		RollerId: "this roller doesn't exist",
		Strategy: Strategy_SINGLE,
	}

	// Check authorization.
	ctx = alogin.FakeStatus(ctx, &notLoggedInStatus)
	res, err := srv.SetStrategy(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")
	ctx = alogin.FakeStatus(ctx, &viewerStatus)
	res, err = srv.SetStrategy(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check error for unknown roller.
	ctx = alogin.FakeStatus(ctx, &editorStatus)
	res, err = srv.SetStrategy(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check results.
	roller.Strategy.(*strategy_mocks.StrategyHistory).On("Add", ctx, strategy.ROLL_STRATEGY_SINGLE, editor, req.Message).Return(nil)
	req.RollerId = roller.Cfg.RollerName
	res, err = srv.SetStrategy(ctx, req)
	require.NoError(t, err)
	st := makeFakeStatus(roller.Cfg)
	manualReqs, err := srv.manualRollDB.GetRecent(roller.Cfg.RollerName, recent_rolls.RecentRollsLength)
	require.NoError(t, err)
	cleanupHistory, err := srv.cleanupDB.History(ctx, roller.Cfg.RollerName, 1)
	require.NoError(t, err)
	expect, err := convertStatus(st, roller.Cfg, roller.Mode.CurrentMode(), roller.Strategy.CurrentStrategy(), manualReqs, cleanupHistory)
	require.NoError(t, err)
	assertdeep.Equal(t, &SetStrategyResponse{
		Status: expect,
	}, res)
}

func TestCreateManualRoll(t *testing.T) {

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &CreateManualRollRequest{
		RollerId: "this roller doesn't exist",
		Revision: "abc123",
	}

	// Check authorization.
	ctx = alogin.FakeStatus(ctx, &notLoggedInStatus)
	res, err := srv.CreateManualRoll(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")
	ctx = alogin.FakeStatus(ctx, &viewerStatus)
	res, err = srv.CreateManualRoll(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check error for unknown roller.
	ctx = alogin.FakeStatus(ctx, &editorStatus)
	res, err = srv.CreateManualRoll(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check results.
	req.RollerId = roller.Cfg.RollerName
	manualReq := &manual.ManualRollRequest{
		RollerName: req.RollerId,
		Revision:   req.Revision,
		Requester:  editor,
		Status:     manual.STATUS_PENDING,
		Timestamp:  firestore.FixTimestamp(timeNowFunc()),
	}
	srv.manualRollDB.(*manual_mocks.DB).On("Put", manualReq).Return(nil)
	res, err = srv.CreateManualRoll(ctx, req)
	require.NoError(t, err)
	expect, err := convertManualRollRequest(manualReq)
	require.NoError(t, err)
	assertdeep.Equal(t, &CreateManualRollResponse{
		Roll: expect,
	}, res)
}

func TestUnthrottle(t *testing.T) {

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &UnthrottleRequest{
		RollerId: "this roller doesn't exist",
	}

	// Check authorization.
	ctx = alogin.FakeStatus(ctx, &notLoggedInStatus)
	res, err := srv.Unthrottle(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")
	ctx = alogin.FakeStatus(ctx, &viewerStatus)
	res, err = srv.Unthrottle(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check error for unknown roller.
	ctx = alogin.FakeStatus(ctx, &editorStatus)
	res, err = srv.Unthrottle(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check results.
	req.RollerId = roller.Cfg.RollerName
	srv.throttle.(*unthrottle_mocks.Throttle).On("Unthrottle", ctx, req.RollerId).Return(nil)
	res, err = srv.Unthrottle(ctx, req)
	require.NoError(t, err)
	assertdeep.Equal(t, &UnthrottleResponse{}, res)
}

func TestAddCleanupRequest(t *testing.T) {
	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &AddCleanupRequestRequest{
		RollerId:      "this roller doesn't exist",
		Justification: "needs cleanup",
	}

	// Check authorization.
	ctx = alogin.FakeStatus(ctx, &notLoggedInStatus)
	res, err := srv.AddCleanupRequest(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")
	ctx = alogin.FakeStatus(ctx, &viewerStatus)
	res, err = srv.AddCleanupRequest(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check error for unknown roller.
	ctx = alogin.FakeStatus(ctx, &editorStatus)
	res, err = srv.AddCleanupRequest(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check results.
	req.RollerId = roller.Cfg.RollerName
	srv.cleanupDB.(*cleanup_mocks.DB).On("RequestCleanup", ctx, &roller_cleanup.CleanupRequest{
		RollerID:      "roller1",
		NeedsCleanup:  true,
		User:          "editor@google.com",
		Timestamp:     currentTime,
		Justification: "needs cleanup",
	}).Return(nil)
	res, err = srv.AddCleanupRequest(ctx, req)
	require.NoError(t, err)
	assertdeep.Equal(t, &AddCleanupRequestResponse{}, res)
}

func TestGetCleanupHistory(t *testing.T) {
	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &GetCleanupHistoryRequest{
		RollerId: "this roller doesn't exist",
		Limit:    1,
	}

	// Check error for unknown roller.
	ctx = alogin.FakeStatus(ctx, &editorStatus)
	res, err := srv.GetCleanupHistory(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check results.
	req.RollerId = roller.Cfg.RollerName
	srv.cleanupDB.(*cleanup_mocks.DB).On("History", ctx, roller.Cfg.RollerName, int(req.Limit)).Return([]*roller_cleanup.CleanupRequest{
		{
			RollerID:      roller.Cfg.RollerName,
			NeedsCleanup:  true,
			User:          "editor@google.com",
			Timestamp:     currentTime,
			Justification: "needs cleanup",
		},
	}, nil)
	res, err = srv.GetCleanupHistory(ctx, req)
	require.NoError(t, err)
	assertdeep.Equal(t, &GetCleanupHistoryResponse{
		History: []*CleanupRequest{
			{
				NeedsCleanup:  true,
				User:          "editor@google.com",
				Timestamp:     timestamppb.New(currentTime),
				Justification: "needs cleanup",
			},
		},
	}, res)
}

func TestConvertMiniStatus(t *testing.T) {

	_, rollers, _ := setup(t)
	r := rollers["roller1"]
	st := r.Status.Get()
	mode, err := convertMode(r.Mode.CurrentMode().Mode)
	require.NoError(t, err)

	// Use Copy to ensure that the test checks all of the fields. Note that it
	// only checks top-level fields and does not dig into member structs.
	actual, err := convertMiniStatus(&st.AutoRollMiniStatus, r.Cfg.RollerName, r.Mode.CurrentMode().Mode, r.Cfg.ChildDisplayName, r.Cfg.ParentDisplayName)
	require.NoError(t, err)
	assertdeep.Copy(t, &AutoRollMiniStatus{
		RollerId:                    r.Cfg.RollerName,
		Mode:                        mode,
		CurrentRollRev:              st.CurrentRollRev,
		LastRollRev:                 st.LastRollRev,
		ChildName:                   st.ChildName,
		ParentName:                  st.ParentName,
		NumFailed:                   int32(st.NumFailedRolls),
		NumBehind:                   int32(st.NumNotRolledCommits),
		Timestamp:                   timestamppb.New(currentTime),
		LastSuccessfulRollTimestamp: timestamppb.New(currentTime),
	}, actual)
}

func TestConvertRollCL(t *testing.T) {

	_, rollers, _ := setup(t)
	r := rollers["roller1"]
	st := r.Status.Get()
	roll := st.LastRoll
	res, err := convertRollCLResult(roll.Result)
	require.NoError(t, err)
	tjs, err := convertTryJobs(roll.TryResults)
	require.NoError(t, err)

	// Use Copy to ensure that the test checks all of the fields. Note that it
	// only checks top-level fields and does not dig into member structs.
	actual, err := convertRollCL(roll)
	require.NoError(t, err)
	assertdeep.Copy(t, &AutoRollCL{
		Id:          fmt.Sprintf("%d", roll.Issue),
		Result:      res,
		Subject:     roll.Subject,
		RollingTo:   roll.RollingTo,
		RollingFrom: roll.RollingFrom,
		Created:     timestamppb.New(roll.Created),
		Modified:    timestamppb.New(roll.Modified),
		TryJobs:     tjs,
	}, actual)
}

func TestConvertTryJob(t *testing.T) {

	_, rollers, _ := setup(t)
	tr := rollers["roller1"].Status.Get().LastRoll.TryResults[0]
	res, err := convertTryJobResult(tr.Result)
	require.NoError(t, err)
	st, err := convertTryJobStatus(tr.Status)
	require.NoError(t, err)

	// Use Copy to ensure that the test checks all of the fields. Note that it
	// only checks top-level fields and does not dig into member structs.
	actual, err := convertTryJob(tr)
	require.NoError(t, err)
	assertdeep.Copy(t, &TryJob{
		Name:     tr.Builder,
		Status:   st,
		Result:   res,
		Url:      tr.Url,
		Category: tr.Category,
	}, actual)
}

func TestConvertModeChange(t *testing.T) {

	_, rollers, _ := setup(t)
	m := rollers["roller1"].Mode.CurrentMode()
	mode, err := convertMode(m.Mode)
	require.NoError(t, err)

	// Use Copy to ensure that the test checks all of the fields. Note that it
	// only checks top-level fields and does not dig into member structs.
	actual, err := convertModeChange(m)
	require.NoError(t, err)
	assertdeep.Copy(t, &ModeChange{
		Message:  m.Message,
		Mode:     mode,
		RollerId: m.Roller,
		Time:     timestamppb.New(m.Time),
		User:     m.User,
	}, actual)
}

func TestConvertStrategyChange(t *testing.T) {

	_, rollers, _ := setup(t)
	s := rollers["roller1"].Strategy.CurrentStrategy()
	strat, err := convertStrategy(s.Strategy)
	require.NoError(t, err)

	// Use Copy to ensure that the test checks all of the fields. Note that it
	// only checks top-level fields and does not dig into member structs.
	actual, err := convertStrategyChange(s)
	require.NoError(t, err)
	assertdeep.Copy(t, &StrategyChange{
		Message:  s.Message,
		RollerId: s.Roller,
		Strategy: strat,
		Time:     timestamppb.New(s.Time),
		User:     s.User,
	}, actual)
}

func TestConvertRevision(t *testing.T) {

	_, rollers, _ := setup(t)
	rev := rollers["roller1"].Status.Get().NotRolledRevisions[0]
	rev.InvalidReason = "bad"

	// Use Copy to ensure that the test checks all of the fields. Note that it
	// only checks top-level fields and does not dig into member structs.
	assertdeep.Copy(t, &Revision{
		Description:   rev.Description,
		Display:       rev.Display,
		Id:            rev.Id,
		InvalidReason: "bad",
		Time:          timestamppb.New(rev.Timestamp),
		Url:           rev.URL,
	}, convertRevision(rev))
}

func TestConvertConfig(t *testing.T) {

	_, rollers, _ := setup(t)
	cfg := rollers["roller1"].Cfg
	cfg.ValidModes = []config.Mode{config.Mode_RUNNING, config.Mode_STOPPED, config.Mode_OFFLINE}

	// Use Copy to ensure that the test checks all of the fields. Note that it
	// only checks top-level fields and does not dig into member structs.
	assertdeep.Copy(t, &AutoRollConfig{
		ChildBugLink:        cfg.ChildBugLink,
		ParentBugLink:       cfg.ParentBugLink,
		ParentWaterfall:     cfg.ParentWaterfall,
		RollerId:            cfg.RollerName,
		SupportsManualRolls: cfg.SupportsManualRolls,
		TimeWindow:          cfg.TimeWindow,
		ValidModes:          []Mode{Mode_RUNNING, Mode_STOPPED, Mode_OFFLINE},
	}, convertConfig(cfg))
}

func TestConvertManualRollRequest(t *testing.T) {

	req := &manual.ManualRollRequest{
		Id:                "999",
		RollerName:        "roller1",
		Revision:          "abc123",
		Requester:         editor,
		Result:            manual.RESULT_SUCCESS,
		Status:            manual.STATUS_COMPLETE,
		Timestamp:         firestore.FixTimestamp(timeNowFunc()),
		Url:               "https://999",
		DryRun:            true,
		Canary:            true,
		NoEmail:           true,
		NoResolveRevision: true,
	}

	// Use Copy to ensure that the test checks all of the fields. Note that it
	// only checks top-level fields and does not dig into member structs.
	actual, err := convertManualRollRequest(req)
	require.NoError(t, err)
	result, err := convertManualRollResult(req.Result)
	require.NoError(t, err)
	status, err := convertManualRollStatus(req.Status)
	require.NoError(t, err)
	assertdeep.Copy(t, &ManualRoll{
		Id:                req.Id,
		RollerId:          req.RollerName,
		Revision:          req.Revision,
		Requester:         req.Requester,
		Result:            result,
		Status:            status,
		Timestamp:         timestamppb.New(req.Timestamp),
		Url:               req.Url,
		DryRun:            true,
		Canary:            true,
		NoEmail:           true,
		NoResolveRevision: true,
	}, actual)
}

func TestConvertStatus(t *testing.T) {
	ctx, rollers, srv := setup(t)
	r := rollers["roller1"]
	cfg := r.Cfg
	st := r.Status.Get()
	mode, err := convertModeChange(r.Mode.CurrentMode())
	require.NoError(t, err)
	strat, err := convertStrategyChange(r.Strategy.CurrentStrategy())
	require.NoError(t, err)
	manualReqs, err := srv.manualRollDB.GetRecent(cfg.RollerName, recent_rolls.RecentRollsLength)
	require.NoError(t, err)
	ms, err := convertMiniStatus(&st.AutoRollMiniStatus, cfg.RollerName, r.Mode.CurrentMode().Mode, cfg.ChildDisplayName, cfg.ParentDisplayName)
	require.NoError(t, err)
	currentRoll, err := convertRollCL(st.CurrentRoll)
	require.NoError(t, err)
	lastRoll, err := convertRollCL(st.LastRoll)
	require.NoError(t, err)
	recentRolls, err := convertRollCLs(st.Recent)
	require.NoError(t, err)
	cleanupHistory, err := srv.cleanupDB.History(ctx, cfg.RollerName, 1)
	require.NoError(t, err)

	// Use Copy to ensure that the test checks all of the fields. Note that it
	// only checks top-level fields and does not dig into member structs.
	actual, err := convertStatus(st, cfg, r.Mode.CurrentMode(), r.Strategy.CurrentStrategy(), manualReqs, cleanupHistory)
	require.NoError(t, err)
	mreqs, err := convertManualRollRequests(manualReqs)
	require.NoError(t, err)
	assertdeep.Copy(t, &AutoRollStatus{
		MiniStatus:         ms,
		Status:             st.Status,
		Config:             convertConfig(cfg),
		FullHistoryUrl:     st.FullHistoryUrl,
		IssueUrlBase:       st.IssueUrlBase,
		Mode:               mode,
		Strategy:           strat,
		NotRolledRevisions: convertRevisions(st.NotRolledRevisions),
		CurrentRoll:        currentRoll,
		LastRoll:           lastRoll,
		RecentRolls:        recentRolls,
		ManualRolls:        mreqs,
		Error:              st.Error,
		ThrottledUntil:     timestamppb.New(time.Unix(st.ThrottledUntil, 0)),
		CleanupRequested: &CleanupRequest{
			NeedsCleanup:  cleanupHistory[0].NeedsCleanup,
			User:          cleanupHistory[0].User,
			Timestamp:     timestamppb.New(cleanupHistory[0].Timestamp),
			Justification: cleanupHistory[0].Justification,
		},
	}, actual)
}

func TestLoadRollerConfigs_DeletesDefunctConfigsAfterDecodeFailure(t *testing.T) {
	mockCurrentTime := currentTime.Add(oldRollerConfigDeletionThreshold + time.Second)
	ctx := context.WithValue(context.Background(), now.ContextKey, mockCurrentTime)
	cdb := &config_db_mocks.DB{}
	cleanupDB := &cleanup_mocks.DB{}
	mdb := &manual_mocks.DB{}
	sdb := &status_mocks.DB{}
	goodRollerID := "good-roller"
	goodRoller := makeRoller(ctx, t, goodRollerID, mdb, cleanupDB)
	badRollerID := "bad-roller"
	badRoller := makeRoller(ctx, t, badRollerID, mdb, cleanupDB)
	cdb.On("GetAll", ctx).Return(nil, &db.FailedDecodeError{
		Err:      fmt.Errorf("failed to decode config"),
		RollerID: badRollerID,
	}).Once()
	sdb.On("Get", ctx, badRollerID).Return(badRoller.Status.Get(), nil)
	cdb.On("Delete", ctx, badRollerID).Return(nil)
	cdb.On("GetAll", ctx).Return([]*config.Config{goodRoller.Cfg}, nil)
	configs, err := loadRollerConfigs(ctx, sdb, cdb)
	require.NoError(t, err)
	require.Len(t, configs, 1)
	require.Equal(t, goodRollerID, configs[0].RollerName)
	cdb.AssertExpectations(t)
	sdb.AssertExpectations(t)
}
