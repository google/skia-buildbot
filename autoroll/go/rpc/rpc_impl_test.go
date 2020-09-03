package rpc

import (
	"context"
	fmt "fmt"
	"sort"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.skia.org/infra/autoroll/go/codereview"
	"go.skia.org/infra/autoroll/go/config_vars"
	"go.skia.org/infra/autoroll/go/manual"
	manual_mocks "go.skia.org/infra/autoroll/go/manual/mocks"
	"go.skia.org/infra/autoroll/go/modes"
	modes_mocks "go.skia.org/infra/autoroll/go/modes/mocks"
	"go.skia.org/infra/autoroll/go/repo_manager"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/status"
	status_mocks "go.skia.org/infra/autoroll/go/status/mocks"
	"go.skia.org/infra/autoroll/go/strategy"
	strategy_mocks "go.skia.org/infra/autoroll/go/strategy/mocks"
	unthrottle_mocks "go.skia.org/infra/autoroll/go/unthrottle/mocks"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/testutils/unittest"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	// Fake user emails.
	viewer = "viewer@google.com"
	editor = "editor@google.com"
	admin  = "admin@google.com"
)

var (
	// Allow fake users.
	viewers = allowed.NewAllowedFromList([]string{viewer, editor, admin})
	editors = allowed.NewAllowedFromList([]string{editor, admin})
	admins  = allowed.NewAllowedFromList([]string{admin})

	// Current at time of writing.
	currentTime = time.Unix(1598467386, 0).UTC()
)

func defaultBranchTmpl(t *testing.T) *config_vars.Template {
	tmpl, err := config_vars.NewTemplate(git.DefaultBranch)
	require.NoError(t, err)
	return tmpl
}

func makeFakeModeChange(cfg *roller.AutoRollerConfig) *modes.ModeChange {
	return &modes.ModeChange{
		Message: "dry run!",
		// We can't use the first enum value, or assertdeep.Copy will fail.
		Mode:   modes.ModeDryRun,
		Roller: cfg.RollerName,
		Time:   timeNowFunc(),
		User:   "me@google.com",
	}
}

func makeFakeStrategyChange(cfg *roller.AutoRollerConfig) *strategy.StrategyChange {
	return &strategy.StrategyChange{
		Message: "set strategy",
		// We can't use the first enum value, or assertdeep.Copy will fail.
		Strategy: strategy.ROLL_STRATEGY_N_BATCH,
		Roller:   cfg.RollerName,
		Time:     timeNowFunc(),
		User:     "you@google.com",
	}
}

func makeFakeManualRollRequest(cfg *roller.AutoRollerConfig) *manual.ManualRollRequest {
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

func makeFakeStatus(cfg *roller.AutoRollerConfig) *status.AutoRollStatus {
	current := makeFakeRoll()
	last := makeFakeRoll()
	return &status.AutoRollStatus{
		AutoRollMiniStatus: status.AutoRollMiniStatus{
			Mode:                modes.ModeRunning,
			CurrentRollRev:      "def456",
			LastRollRev:         "abc123",
			NumFailedRolls:      1,
			NumNotRolledCommits: 2,
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

func makeRoller(ctx context.Context, t *testing.T, name string, mdb *manual_mocks.DB) *AutoRoller {
	cfg := &roller.AutoRollerConfig{
		ChildDisplayName:  name + "_child",
		ParentDisplayName: name + "_parent",
		ParentWaterfall:   "https://parent",
		RollerName:        name,
		NoCheckoutDEPSRepoManager: &repo_manager.NoCheckoutDEPSRepoManagerConfig{
			NoCheckoutRepoManagerConfig: repo_manager.NoCheckoutRepoManagerConfig{
				CommonRepoManagerConfig: repo_manager.CommonRepoManagerConfig{
					ChildBranch:  defaultBranchTmpl(t),
					ChildPath:    "path/to/child",
					ParentBranch: defaultBranchTmpl(t),
					ParentRepo:   "https://fake.parent",
				},
			},
			Gerrit: &codereview.GerritConfig{},
			// URL of the child repo.
			ChildRepo: "https://fake.child",
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
	mdb.On("GetRecent", cfg.RollerName, 2 /* number of not-rolled revs in fake status */).Return([]*manual.ManualRollRequest{manualReq}, nil)

	return &AutoRoller{
		Cfg:      cfg,
		Mode:     modeHistory,
		Status:   statusCache,
		Strategy: strategyHistory,
	}
}

func setup(t *testing.T) (context.Context, map[string]*AutoRoller, *autoRollServerImpl) {
	timeNowFunc = func() time.Time {
		return currentTime
	}
	ctx := context.Background()
	mdb := &manual_mocks.DB{}
	r1 := makeRoller(ctx, t, "roller1", mdb)
	r2 := makeRoller(ctx, t, "roller2", mdb)
	rollers := map[string]*AutoRoller{
		r1.Cfg.RollerName: r1,
		r2.Cfg.RollerName: r2,
	}
	srv := newAutoRollServerImpl(rollers, mdb, &unthrottle_mocks.Throttle{}, viewers, editors, admins)
	return ctx, rollers, srv
}

func TestGetRollers(t *testing.T) {
	unittest.SmallTest(t)

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
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

func TestGetMiniStatus(t *testing.T) {
	unittest.SmallTest(t)

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &GetMiniStatusRequest{
		RollerId: "this roller doesn't exist",
	}

	// Check authorization.
	mockUser := ""
	srv.MockGetUserForTesting(func(ctx context.Context) string {
		return mockUser
	})
	res, err := srv.GetMiniStatus(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized viewer")
	mockUser = "no-access@google.com"
	res, err = srv.GetMiniStatus(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"no-access@google.com\" is not an authorized viewer")

	// Check error for unknown roller.
	mockUser = viewer
	res, err = srv.GetMiniStatus(ctx, req)
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
			ChildName:      roller.Cfg.ChildDisplayName,
			Mode:           mode,
			ParentName:     roller.Cfg.ParentDisplayName,
			RollerId:       roller.Cfg.RollerName,
			CurrentRollRev: "def456",
			LastRollRev:    "abc123",
			NumFailed:      1,
			NumBehind:      2,
		},
	}, res)
}

func TestGetStatus(t *testing.T) {
	unittest.SmallTest(t)

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &GetStatusRequest{
		RollerId: "this roller doesn't exist",
	}

	// Check authorization.
	mockUser := ""
	srv.MockGetUserForTesting(func(ctx context.Context) string {
		return mockUser
	})
	res, err := srv.GetStatus(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized viewer")
	mockUser = "no-access@google.com"
	res, err = srv.GetStatus(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"no-access@google.com\" is not an authorized viewer")

	// Check error for unknown roller.
	mockUser = viewer
	res, err = srv.GetStatus(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check results.
	req.RollerId = roller.Cfg.RollerName
	res, err = srv.GetStatus(ctx, req)
	require.NoError(t, err)
	st := makeFakeStatus(roller.Cfg)
	manualReqs, err := srv.manualRollDB.GetRecent(roller.Cfg.RollerName, len(st.NotRolledRevisions))
	expect, err := convertStatus(st, roller.Cfg, roller.Mode.CurrentMode(), roller.Strategy.CurrentStrategy(), manualReqs)
	require.NoError(t, err)
	assertdeep.Equal(t, &GetStatusResponse{
		Status: expect,
	}, res)
}

func TestSetMode(t *testing.T) {
	unittest.SmallTest(t)

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &SetModeRequest{
		Message:  "new mode",
		Mode:     Mode_DRY_RUN,
		RollerId: "this roller doesn't exist",
	}

	// Check authorization.
	mockUser := ""
	srv.MockGetUserForTesting(func(ctx context.Context) string {
		return mockUser
	})
	res, err := srv.SetMode(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")
	mockUser = viewer
	res, err = srv.SetMode(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check error for unknown roller.
	mockUser = editor
	res, err = srv.SetMode(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check results.
	roller.Mode.(*modes_mocks.ModeHistory).On("Add", ctx, modes.ModeDryRun, editor, req.Message).Return(nil)
	req.RollerId = roller.Cfg.RollerName
	res, err = srv.SetMode(ctx, req)
	require.NoError(t, err)
	st := makeFakeStatus(roller.Cfg)
	manualReqs, err := srv.manualRollDB.GetRecent(roller.Cfg.RollerName, len(st.NotRolledRevisions))
	expect, err := convertStatus(st, roller.Cfg, roller.Mode.CurrentMode(), roller.Strategy.CurrentStrategy(), manualReqs)
	require.NoError(t, err)
	assertdeep.Equal(t, &SetModeResponse{
		Status: expect,
	}, res)
}

func TestSetStrategy(t *testing.T) {
	unittest.SmallTest(t)

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &SetStrategyRequest{
		Message:  "new strategy",
		RollerId: "this roller doesn't exist",
		Strategy: Strategy_SINGLE,
	}

	// Check authorization.
	mockUser := ""
	srv.MockGetUserForTesting(func(ctx context.Context) string {
		return mockUser
	})
	res, err := srv.SetStrategy(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")
	mockUser = viewer
	res, err = srv.SetStrategy(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check error for unknown roller.
	mockUser = editor
	res, err = srv.SetStrategy(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check results.
	roller.Strategy.(*strategy_mocks.StrategyHistory).On("Add", ctx, strategy.ROLL_STRATEGY_SINGLE, editor, req.Message).Return(nil)
	req.RollerId = roller.Cfg.RollerName
	res, err = srv.SetStrategy(ctx, req)
	require.NoError(t, err)
	st := makeFakeStatus(roller.Cfg)
	manualReqs, err := srv.manualRollDB.GetRecent(roller.Cfg.RollerName, len(st.NotRolledRevisions))
	expect, err := convertStatus(st, roller.Cfg, roller.Mode.CurrentMode(), roller.Strategy.CurrentStrategy(), manualReqs)
	require.NoError(t, err)
	assertdeep.Equal(t, &SetStrategyResponse{
		Status: expect,
	}, res)
}

func TestCreateManualRoll(t *testing.T) {
	unittest.SmallTest(t)

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &CreateManualRollRequest{
		RollerId: "this roller doesn't exist",
		Revision: "abc123",
	}

	// Check authorization.
	mockUser := ""
	srv.MockGetUserForTesting(func(ctx context.Context) string {
		return mockUser
	})
	res, err := srv.CreateManualRoll(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")
	mockUser = viewer
	res, err = srv.CreateManualRoll(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check error for unknown roller.
	mockUser = editor
	res, err = srv.CreateManualRoll(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error not_found: Unknown roller")

	// Check results.
	req.RollerId = roller.Cfg.RollerName
	manualReq := &manual.ManualRollRequest{
		RollerName: req.RollerId,
		Revision:   req.Revision,
		Requester:  mockUser,
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
	unittest.SmallTest(t)

	// Setup, mocks.
	ctx, rollers, srv := setup(t)
	roller := rollers["roller1"]
	req := &UnthrottleRequest{
		RollerId: "this roller doesn't exist",
	}

	// Check authorization.
	mockUser := ""
	srv.MockGetUserForTesting(func(ctx context.Context) string {
		return mockUser
	})
	res, err := srv.Unthrottle(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"\" is not an authorized editor")
	mockUser = viewer
	res, err = srv.Unthrottle(ctx, req)
	require.Nil(t, res)
	require.EqualError(t, err, "twirp error permission_denied: \"viewer@google.com\" is not an authorized editor")

	// Check error for unknown roller.
	mockUser = editor
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

func TestConvertMiniStatus(t *testing.T) {
	unittest.SmallTest(t)

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
		RollerId:       r.Cfg.RollerName,
		Mode:           mode,
		CurrentRollRev: st.CurrentRollRev,
		LastRollRev:    st.LastRollRev,
		ChildName:      st.ChildName,
		ParentName:     st.ParentName,
		NumFailed:      int32(st.NumFailedRolls),
		NumBehind:      int32(st.NumNotRolledCommits),
	}, actual)
}

func TestConvertRollCL(t *testing.T) {
	unittest.SmallTest(t)

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
	unittest.SmallTest(t)

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
	unittest.SmallTest(t)

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
	unittest.SmallTest(t)

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
	unittest.SmallTest(t)

	_, rollers, _ := setup(t)
	rev := rollers["roller1"].Status.Get().NotRolledRevisions[0]

	// Use Copy to ensure that the test checks all of the fields. Note that it
	// only checks top-level fields and does not dig into member structs.
	assertdeep.Copy(t, &Revision{
		Description: rev.Description,
		Display:     rev.Display,
		Id:          rev.Id,
		Time:        timestamppb.New(rev.Timestamp),
		Url:         rev.URL,
	}, convertRevision(rev))
}

func TestConvertConfig(t *testing.T) {
	unittest.SmallTest(t)

	_, rollers, _ := setup(t)
	cfg := rollers["roller1"].Cfg

	// Use Copy to ensure that the test checks all of the fields. Note that it
	// only checks top-level fields and does not dig into member structs.
	assertdeep.Copy(t, &AutoRollConfig{
		ParentWaterfall:     cfg.ParentWaterfall,
		RollerId:            cfg.RollerName,
		SupportsManualRolls: cfg.SupportsManualRolls,
		TimeWindow:          cfg.TimeWindow,
	}, convertConfig(cfg))
}

func TestConvertManualRollRequest(t *testing.T) {
	unittest.SmallTest(t)

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
		NoEmail:           true,
		NoResolveRevision: true,
	}, actual)
}

func TestConvertStatus(t *testing.T) {
	unittest.SmallTest(t)

	_, rollers, srv := setup(t)
	r := rollers["roller1"]
	cfg := r.Cfg
	st := r.Status.Get()
	mode, err := convertModeChange(r.Mode.CurrentMode())
	require.NoError(t, err)
	strat, err := convertStrategyChange(r.Strategy.CurrentStrategy())
	require.NoError(t, err)
	manualReqs, err := srv.manualRollDB.GetRecent(cfg.RollerName, len(st.NotRolledRevisions))
	require.NoError(t, err)
	ms, err := convertMiniStatus(&st.AutoRollMiniStatus, cfg.RollerName, r.Mode.CurrentMode().Mode, cfg.ChildDisplayName, cfg.ParentDisplayName)
	require.NoError(t, err)
	currentRoll, err := convertRollCL(st.CurrentRoll)
	require.NoError(t, err)
	lastRoll, err := convertRollCL(st.LastRoll)
	require.NoError(t, err)
	recentRolls, err := convertRollCLs(st.Recent)
	require.NoError(t, err)

	// Use Copy to ensure that the test checks all of the fields. Note that it
	// only checks top-level fields and does not dig into member structs.
	actual, err := convertStatus(st, cfg, r.Mode.CurrentMode(), r.Strategy.CurrentStrategy(), manualReqs)
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
	}, actual)
}
