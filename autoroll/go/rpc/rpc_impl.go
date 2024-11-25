package rpc

import (
	"context"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"

	"github.com/twitchtv/twirp"
	"go.skia.org/infra/autoroll/go/config"
	"go.skia.org/infra/autoroll/go/config/db"
	"go.skia.org/infra/autoroll/go/manual"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/recent_rolls"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/roller_cleanup"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/autoroll/go/unthrottle"
	"go.skia.org/infra/go/alogin"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/twirp_auth2"
	"go.skia.org/infra/go/util"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Generate the go code from the protocol buffer definitions.
//go:generate bazelisk run --config=mayberemote //:protoc -- --go_opt=paths=source_relative --twirp_out=. --go_out=. ./rpc.proto
//go:generate mv ./go.skia.org/infra/autoroll/go/rpc/rpc.twirp.go ./rpc.twirp.go
//go:generate rm -rf ./go.skia.org
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w rpc.pb.go
//go:generate bazelisk run --config=mayberemote //:goimports "--run_under=cd $PWD &&" -- -w rpc.twirp.go
//go:generate bazelisk run --config=mayberemote //:protoc -- --twirp_typescript_out=../../modules/rpc ./rpc.proto

// timeNowFunc allows tests to mock out time.Now() for testing.
var timeNowFunc = time.Now

// loadRollersFunc allows tests to mock out loadRollers() for testing.
var loadRollersFunc = loadRollers

// AutoRollServer implements AutoRollRPCs.
type AutoRollServer struct {
	*twirp_auth2.AuthHelper
	cancelPolling context.CancelFunc
	cleanupDB     roller_cleanup.DB
	handler       http.Handler
	manualRollDB  manual.DB
	throttle      unthrottle.Throttle
	rollers       map[string]*AutoRoller
	rollersMtx    sync.RWMutex
	rollsDB       recent_rolls.DB
}

// GetHandler returns the http.Handler for this AutoRollServer.
func (s *AutoRollServer) GetHandler() http.Handler {
	return s.handler
}

// NewAutoRollServer returns an AutoRollServer instance.
// If configRefreshInterval is zero, the configs are not refreshed.
func NewAutoRollServer(ctx context.Context, statusDB status.DB, configDB db.DB, rollsDB recent_rolls.DB, manualRollDB manual.DB, cleanupDB roller_cleanup.DB, throttle unthrottle.Throttle, configRefreshInterval time.Duration, plogin alogin.Login) (*AutoRollServer, error) {
	rollers, cancelPolling, err := loadRollersFunc(ctx, statusDB, configDB)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to load roller configs from DB")
	}
	srv := &AutoRollServer{
		AuthHelper:    twirp_auth2.New(),
		cancelPolling: cancelPolling,
		cleanupDB:     cleanupDB,
		manualRollDB:  manualRollDB,
		throttle:      throttle,
		rollers:       rollers,
		rollsDB:       rollsDB,
	}

	srv.handler = alogin.StatusMiddleware(plogin)(NewAutoRollServiceServer(srv, nil))
	if configRefreshInterval != time.Duration(0) {
		go util.RepeatCtx(ctx, configRefreshInterval, func(ctx context.Context) {
			rollers, cancelPolling, err = loadRollersFunc(ctx, statusDB, configDB)
			if err != nil {
				sklog.Errorf("Failed to refresh rollers: %s", err)
				return
			}
			srv.rollersMtx.Lock()
			defer srv.rollersMtx.Unlock()
			srv.cancelPolling()
			srv.rollers = rollers
			srv.cancelPolling = cancelPolling
		})
	}
	return srv, nil
}

// oldRollerConfigDeletionThreshold indicates that we'll delete a roller config
// from the DB if we fail to decode it and it's been two weeks since it last
// reported in. We assume that if the roller hasn't reported in for two weeks it
// has been removed.
const oldRollerConfigDeletionThreshold = 2 * 7 * 24 * time.Hour

// loadRollerConfigs loads the roller configs from the config DB. If it
// encounters errors for rollers which haven't reported their status for two
// weeks, it may delete their configs from the DB.
func loadRollerConfigs(ctx context.Context, statusDB status.DB, configDB db.DB) ([]*config.Config, error) {
	var configs []*config.Config
	var err error
	for {
		configs, err = configDB.GetAll(ctx)
		if err == nil {
			return configs, nil
		} else {
			// The config format might have changed, causing a failure to decode
			// configs for defunct rollers. Determine whether the roller is likely
			// turned down and delete the config from the DB if so.
			if failedDecodeRoller, ok := db.IsFailedDecode(err); ok {
				status, getStatusErr := statusDB.Get(ctx, failedDecodeRoller)
				if getStatusErr != nil {
					return nil, skerr.Wrapf(err, "failed to decode roller config %s and failed to retrieve status: %s", failedDecodeRoller, getStatusErr)
				}
				lastCheckin := now.Now(ctx).Sub(status.Timestamp)
				if lastCheckin > oldRollerConfigDeletionThreshold {
					sklog.Errorf("Failed to decode config for %s; last checked in %s ago; deleting config...", failedDecodeRoller, lastCheckin)
					if err := configDB.Delete(ctx, failedDecodeRoller); err != nil {
						return nil, skerr.Wrapf(err, "failed to decode config for defunct roller %s and failed to delete config from the DB", failedDecodeRoller)
					}
				} else {
					return nil, skerr.Wrap(err)
				}
			} else {
				return nil, skerr.Wrap(err)
			}
		}
	}
}

// loadRollers loads the roller configs from the config DB and creates the
// various databases used for each roller.  Returns a map containing the rollers
// themselves and a context.CancelFunc which can be used to stop the polling
// loops for the rollers, eg. when loadRollers is to be called again.
func loadRollers(ctx context.Context, statusDB status.DB, configDB db.DB) (rv map[string]*AutoRoller, rvCancel context.CancelFunc, rvErr error) {
	configs, err := loadRollerConfigs(ctx, statusDB, configDB)
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}

	rollers := make(map[string]*AutoRoller, len(configs))
	// Use a cancellable context so that we can restart the polling loops when
	// we reload the rollers next time.
	cancellableCtx, cancel := context.WithCancel(ctx)
	defer func() {
		// If something went wrong, cancel the polling loops to avoid a
		// goroutine leak.
		if rvErr != nil {
			cancel()
		}
	}()
	for _, cfg := range configs {
		// Set up DBs for the roller.
		cfg := cfg // Capture loop variable for use by goroutines.
		arbMode, err := modes.NewDatastoreModeHistory(ctx, cfg.RollerName)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		go util.RepeatCtx(cancellableCtx, 10*time.Second, func(_ context.Context) {
			if err := arbMode.Update(ctx); err != nil {
				sklog.Errorf("Failed to retrieve mode history for %s: %s", cfg.RollerName, err)
			}
		})
		arbStatus, err := status.NewCache(ctx, statusDB, cfg.RollerName)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		go util.RepeatCtx(cancellableCtx, 10*time.Second, func(_ context.Context) {
			if err := arbStatus.Update(ctx); err != nil {
				sklog.Errorf("Failed to retrieve status for %s: %s", cfg.RollerName, err)
			}
		})
		arbStrategy, err := strategy.NewDatastoreStrategyHistory(ctx, cfg.RollerName, cfg.ValidStrategies())
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		go util.RepeatCtx(cancellableCtx, 10*time.Second, func(_ context.Context) {
			if err := arbStrategy.Update(ctx); err != nil {
				sklog.Errorf("Failed to retrieve strategy history for %s: %s", cfg.RollerName, err)
			}
		})
		rollers[cfg.RollerName] = &AutoRoller{
			Cfg:      cfg,
			Mode:     arbMode,
			Status:   arbStatus,
			Strategy: arbStrategy,
		}
	}
	return rollers, cancel, nil
}

// GetRoller retrieves the given roller.
func (s *AutoRollServer) GetRoller(roller string) (*AutoRoller, error) {
	s.rollersMtx.RLock()
	defer s.rollersMtx.RUnlock()
	rv, ok := s.rollers[roller]
	if !ok {
		return nil, twirp.NewError(twirp.NotFound, "Unknown roller")
	}
	return rv, nil
}

// Helper for sorting AutoRollMiniStatuses.
type autoRollMiniStatusSlice []*AutoRollMiniStatus

func (s autoRollMiniStatusSlice) Len() int {
	return len(s)
}

func (s autoRollMiniStatusSlice) Less(a, b int) bool {
	return s[a].RollerId < s[b].RollerId
}

func (s autoRollMiniStatusSlice) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}

// GetRollers implements AutoRollRPCs.
func (s *AutoRollServer) GetRollers(ctx context.Context, req *GetRollersRequest) (*GetRollersResponse, error) {
	s.rollersMtx.RLock()
	defer s.rollersMtx.RUnlock()
	statuses := make([]*AutoRollMiniStatus, 0, len(s.rollers))
	for name, roller := range s.rollers {
		mc := roller.Mode.CurrentMode()
		mode := modes.ModeRunning
		if mc != nil {
			mode = mc.Mode
		}
		st, err := convertMiniStatus(roller.Status.GetMini(), name, mode, roller.Cfg.ChildDisplayName, roller.Cfg.ParentDisplayName)
		if err != nil {
			return nil, err
		}
		statuses = append(statuses, st)
	}
	// Sort for testing.
	sort.Sort(autoRollMiniStatusSlice(statuses))
	return &GetRollersResponse{
		Rollers: statuses,
	}, nil
}

// GetRolls implements AutoRollRPCs.
func (s *AutoRollServer) GetRolls(ctx context.Context, req *GetRollsRequest) (*GetRollsResponse, error) {
	if _, err := s.GetRoller(req.RollerId); err != nil {
		return nil, err
	}
	rolls, cursor, err := s.rollsDB.GetRolls(ctx, req.RollerId, req.Cursor)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rollsConv, err := convertRollCLs(rolls)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return &GetRollsResponse{
		Rolls:  rollsConv,
		Cursor: cursor,
	}, nil
}

// GetMiniStatus implements AutoRollRPCs.
func (s *AutoRollServer) GetMiniStatus(ctx context.Context, req *GetMiniStatusRequest) (*GetMiniStatusResponse, error) {
	roller, err := s.GetRoller(req.RollerId)
	if err != nil {
		return nil, err
	}
	ms, err := convertMiniStatus(roller.Status.GetMini(), req.RollerId, roller.Mode.CurrentMode().Mode, roller.Cfg.ChildDisplayName, roller.Cfg.ParentDisplayName)
	if err != nil {
		return nil, err
	}
	return &GetMiniStatusResponse{
		Status: ms,
	}, nil
}

// getStatus retrieves the status for the given roller.
func (s *AutoRollServer) getStatus(ctx context.Context, rollerID string) (*AutoRollStatus, error) {
	roller, err := s.GetRoller(rollerID)
	if err != nil {
		return nil, err
	}
	st := roller.Status.Get()
	var manualReqs []*manual.ManualRollRequest
	if roller.Cfg.SupportsManualRolls {
		manualReqs, err = s.manualRollDB.GetRecent(roller.Cfg.RollerName, recent_rolls.RecentRollsLength)
		if err != nil {
			return nil, err
		}
	}
	cleanup, err := s.cleanupDB.History(ctx, rollerID, 1)
	if err != nil {
		return nil, err
	}
	return convertStatus(st, roller.Cfg, roller.Mode.CurrentMode(), roller.Strategy.CurrentStrategy(), manualReqs, cleanup)
}

// GetStatus implements AutoRollRPCs.
func (s *AutoRollServer) GetStatus(ctx context.Context, req *GetStatusRequest) (*GetStatusResponse, error) {
	st, err := s.getStatus(ctx, req.RollerId)
	if err != nil {
		return nil, err
	}
	return &GetStatusResponse{
		Status: st,
	}, nil
}

// SetMode implements AutoRollRPCs.
func (s *AutoRollServer) SetMode(ctx context.Context, req *SetModeRequest) (*SetModeResponse, error) {
	// Verify that the user has edit access.
	user, err := s.GetEditor(ctx)
	if err != nil {
		return nil, err
	}
	roller, err := s.GetRoller(req.RollerId)
	if err != nil {
		return nil, err
	}
	var mode string
	switch req.Mode {
	case Mode_RUNNING:
		mode = modes.ModeRunning
	case Mode_STOPPED:
		mode = modes.ModeStopped
	case Mode_DRY_RUN:
		mode = modes.ModeDryRun
	case Mode_OFFLINE:
		mode = modes.ModeOffline
	default:
		return nil, twirp.InvalidArgumentError("mode", "invalid mode")
	}
	if len(roller.Cfg.ValidModes) > 0 {
		// Note: this assumes that the Mode enums in rpc.proto and config.proto
		// are in sync.
		modeConv := config.Mode(req.Mode)
		isValidMode := false
		validModeStrs := make([]string, 0, len(roller.Cfg.ValidModes))
		for _, validMode := range roller.Cfg.ValidModes {
			validModeStrs = append(validModeStrs, validMode.String())
			if modeConv == validMode {
				isValidMode = true
			}
		}
		if !isValidMode {
			return nil, twirp.InvalidArgumentError("mode", fmt.Sprintf("requested mode is not allowed for this roller; valid modes: %v", validModeStrs))
		}
	}
	if err := roller.Mode.Add(ctx, mode, user, req.Message); err != nil {
		return nil, err
	}
	st, err := s.getStatus(ctx, req.RollerId)
	if err != nil {
		return nil, err
	}
	return &SetModeResponse{
		Status: st,
	}, nil
}

// GetModeHistory implements AutoRollRPCs.
func (s *AutoRollServer) GetModeHistory(ctx context.Context, req *GetModeHistoryRequest) (*GetModeHistoryResponse, error) {
	roller, err := s.GetRoller(req.RollerId)
	if err != nil {
		return nil, err
	}
	history, nextOffset, err := roller.Mode.GetHistory(ctx, int(req.Offset))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	historyConv := make([]*ModeChange, 0, len(history))
	for _, entry := range history {
		mc, err := convertModeChange(entry)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		historyConv = append(historyConv, mc)
	}
	return &GetModeHistoryResponse{
		History:    historyConv,
		NextOffset: int32(nextOffset),
	}, nil
}

// SetStrategy implements AutoRollRPCs.
func (s *AutoRollServer) SetStrategy(ctx context.Context, req *SetStrategyRequest) (*SetStrategyResponse, error) {
	// Verify that the user has edit access.
	user, err := s.GetEditor(ctx)
	if err != nil {
		return nil, err
	}
	roller, err := s.GetRoller(req.RollerId)
	if err != nil {
		return nil, err
	}
	var strat string
	switch req.Strategy {
	case Strategy_BATCH:
		strat = strategy.ROLL_STRATEGY_BATCH
	case Strategy_N_BATCH:
		strat = strategy.ROLL_STRATEGY_N_BATCH
	case Strategy_SINGLE:
		strat = strategy.ROLL_STRATEGY_SINGLE
	default:
		return nil, twirp.InvalidArgumentError("strategy", "invalid strategy")
	}
	if err := roller.Strategy.Add(ctx, strat, user, req.Message); err != nil {
		return nil, err
	}
	st, err := s.getStatus(ctx, req.RollerId)
	if err != nil {
		return nil, err
	}
	return &SetStrategyResponse{
		Status: st,
	}, nil
}

// GetStrategyHistory implements AutoRollRPCs.
func (s *AutoRollServer) GetStrategyHistory(ctx context.Context, req *GetStrategyHistoryRequest) (*GetStrategyHistoryResponse, error) {
	roller, err := s.GetRoller(req.RollerId)
	if err != nil {
		return nil, err
	}
	history, nextOffset, err := roller.Strategy.GetHistory(ctx, int(req.Offset))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	historyConv := make([]*StrategyChange, 0, len(history))
	for _, entry := range history {
		mc, err := convertStrategyChange(entry)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		historyConv = append(historyConv, mc)
	}
	return &GetStrategyHistoryResponse{
		History:    historyConv,
		NextOffset: int32(nextOffset),
	}, nil
}

// CreateManualRoll implements AutoRollRPCs.
func (s *AutoRollServer) CreateManualRoll(ctx context.Context, req *CreateManualRollRequest) (*CreateManualRollResponse, error) {
	// Verify that the user has edit access.
	user, err := s.GetEditor(ctx)
	if err != nil {
		return nil, err
	}
	// Check that the roller exists.
	if _, err := s.GetRoller(req.RollerId); err != nil {
		return nil, err
	}
	m := &manual.ManualRollRequest{
		RollerName: req.RollerId,
		Revision:   req.Revision,
		Requester:  user,
		DryRun:     req.DryRun,
	}
	m.Status = manual.STATUS_PENDING
	m.Timestamp = firestore.FixTimestamp(timeNowFunc())
	if err := s.manualRollDB.Put(m); err != nil {
		return nil, err
	}
	resp, err := convertManualRollRequest(m)
	if err != nil {
		return nil, err
	}
	return &CreateManualRollResponse{
		Roll: resp,
	}, nil
}

// Unthrottle implements AutoRollRPCs.
func (s *AutoRollServer) Unthrottle(ctx context.Context, req *UnthrottleRequest) (*UnthrottleResponse, error) {
	// Verify that the user has edit access.
	if _, err := s.GetEditor(ctx); err != nil {
		return nil, err
	}
	// Check that the roller exists.
	if _, err := s.GetRoller(req.RollerId); err != nil {
		return nil, err
	}
	if err := s.throttle.Unthrottle(ctx, req.RollerId); err != nil {
		return nil, err
	}
	return &UnthrottleResponse{}, nil
}

// AddCleanupRequest implements AutoRollRPCs.
func (s *AutoRollServer) AddCleanupRequest(ctx context.Context, req *AddCleanupRequestRequest) (*AddCleanupRequestResponse, error) {
	// Verify that the user has edit access.
	user, err := s.GetEditor(ctx)
	if err != nil {
		return nil, err
	}
	// Check that the roller exists.
	if _, err := s.GetRoller(req.RollerId); err != nil {
		return nil, err
	}
	cr := &roller_cleanup.CleanupRequest{
		RollerID:      req.RollerId,
		NeedsCleanup:  true,
		User:          user,
		Timestamp:     firestore.FixTimestamp(timeNowFunc()),
		Justification: req.Justification,
	}
	if err := s.cleanupDB.RequestCleanup(ctx, cr); err != nil {
		return nil, err
	}
	return &AddCleanupRequestResponse{}, nil
}

// GetCleanupHistory implements AutoRollRPCs.
func (s *AutoRollServer) GetCleanupHistory(ctx context.Context, req *GetCleanupHistoryRequest) (*GetCleanupHistoryResponse, error) {
	// Check that the roller exists.
	if _, err := s.GetRoller(req.RollerId); err != nil {
		return nil, err
	}
	history, err := s.cleanupDB.History(ctx, req.RollerId, int(req.Limit))
	if err != nil {
		return nil, err
	}
	rv := make([]*CleanupRequest, 0, len(history))
	for _, cr := range history {
		rv = append(rv, &CleanupRequest{
			NeedsCleanup:  cr.NeedsCleanup,
			User:          cr.User,
			Timestamp:     timestamppb.New(cr.Timestamp),
			Justification: cr.Justification,
		})
	}
	return &GetCleanupHistoryResponse{
		History: rv,
	}, nil
}

// AutoRoller provides interactions with a single roller.
type AutoRoller struct {
	Cfg *config.Config

	// Interactions with the roller through the DB.
	Mode     modes.ModeHistory
	Status   *status.Cache
	Strategy strategy.StrategyHistory
}

func convertMiniStatus(inp *status.AutoRollMiniStatus, roller, mode, childName, parentName string) (*AutoRollMiniStatus, error) {
	m, err := convertMode(mode)
	if err != nil {
		return nil, err
	}
	return &AutoRollMiniStatus{
		RollerId:                    roller,
		Mode:                        m,
		CurrentRollRev:              inp.CurrentRollRev,
		LastRollRev:                 inp.LastRollRev,
		ChildName:                   childName,
		ParentName:                  parentName,
		NumFailed:                   int32(inp.NumFailedRolls),
		NumBehind:                   int32(inp.NumNotRolledCommits),
		Timestamp:                   timestamppb.New(inp.Timestamp),
		LastSuccessfulRollTimestamp: timestamppb.New(inp.LastSuccessfulRollTimestamp),
	}, nil
}

func convertRollCLs(inp []*autoroll.AutoRollIssue) ([]*AutoRollCL, error) {
	rv := make([]*AutoRollCL, 0, len(inp))
	for _, v := range inp {
		cl, err := convertRollCL(v)
		if err != nil {
			return nil, err
		}
		rv = append(rv, cl)
	}
	return rv, nil
}

func convertRollCLResult(res string) (AutoRollCL_Result, error) {
	switch res {
	case autoroll.ROLL_RESULT_IN_PROGRESS:
		return AutoRollCL_IN_PROGRESS, nil
	case autoroll.ROLL_RESULT_SUCCESS:
		return AutoRollCL_SUCCESS, nil
	case autoroll.ROLL_RESULT_FAILURE:
		return AutoRollCL_FAILURE, nil
	case autoroll.ROLL_RESULT_DRY_RUN_IN_PROGRESS:
		return AutoRollCL_DRY_RUN_IN_PROGRESS, nil
	case autoroll.ROLL_RESULT_DRY_RUN_SUCCESS:
		return AutoRollCL_DRY_RUN_SUCCESS, nil
	case autoroll.ROLL_RESULT_DRY_RUN_FAILURE:
		return AutoRollCL_DRY_RUN_FAILURE, nil
	default:
		return -1, twirp.InternalError(fmt.Sprintf("invalid roll result %q", res))
	}
}

func convertRollCL(inp *autoroll.AutoRollIssue) (*AutoRollCL, error) {
	if inp == nil {
		return nil, nil
	}
	tjs, err := convertTryJobs(inp.TryResults)
	if err != nil {
		return nil, err
	}
	res, err := convertRollCLResult(inp.Result)
	if err != nil {
		return nil, err
	}
	return &AutoRollCL{
		Id:          fmt.Sprintf("%d", inp.Issue),
		Result:      res,
		Subject:     inp.Subject,
		RollingTo:   inp.RollingTo,
		RollingFrom: inp.RollingFrom,
		Created:     timestamppb.New(inp.Created),
		Modified:    timestamppb.New(inp.Modified),
		TryJobs:     tjs,
	}, nil
}

func convertTryJobs(inp []*autoroll.TryResult) ([]*TryJob, error) {
	if inp == nil {
		return nil, nil
	}
	rv := make([]*TryJob, 0, len(inp))
	for _, v := range inp {
		tj, err := convertTryJob(v)
		if err != nil {
			return nil, err
		}
		rv = append(rv, tj)
	}
	return rv, nil
}

func convertTryJobStatus(st string) (TryJob_Status, error) {
	v, ok := TryJob_Status_value[st]
	if !ok {
		return -1, twirp.InternalError(fmt.Sprintf("invalid tryjob status %q", st))
	}
	return TryJob_Status(v), nil
}

func convertTryJobResult(st string) (TryJob_Result, error) {
	// Special case: "" -> UNKNOWN
	if st == "" {
		return TryJob_UNKNOWN, nil
	}
	v, ok := TryJob_Result_value[st]
	if !ok {
		return -1, twirp.InternalError(fmt.Sprintf("invalid tryjob result %q", st))
	}
	return TryJob_Result(v), nil
}

func convertTryJob(inp *autoroll.TryResult) (*TryJob, error) {
	st, err := convertTryJobStatus(inp.Status)
	if err != nil {
		return nil, err
	}
	res, err := convertTryJobResult(inp.Result)
	if err != nil {
		return nil, err
	}
	return &TryJob{
		Name:     inp.Builder,
		Status:   st,
		Result:   res,
		Url:      inp.Url,
		Category: inp.Category,
	}, nil
}

func convertMode(m string) (Mode, error) {
	switch m {
	case modes.ModeRunning:
		return Mode_RUNNING, nil
	case modes.ModeStopped:
		return Mode_STOPPED, nil
	case modes.ModeDryRun:
		return Mode_DRY_RUN, nil
	case modes.ModeOffline:
		return Mode_OFFLINE, nil
	default:
		return -1, twirp.InternalError(fmt.Sprintf("invalid mode %q", m))
	}
}

func convertModeChange(inp *modes.ModeChange) (*ModeChange, error) {
	mode, err := convertMode(inp.Mode)
	if err != nil {
		return nil, err
	}
	return &ModeChange{
		Message:  inp.Message,
		Mode:     mode,
		RollerId: inp.Roller,
		Time:     timestamppb.New(inp.Time),
		User:     inp.User,
	}, nil
}

func convertStrategy(s string) (Strategy, error) {
	switch s {
	case strategy.ROLL_STRATEGY_BATCH:
		return Strategy_BATCH, nil
	case strategy.ROLL_STRATEGY_N_BATCH:
		return Strategy_N_BATCH, nil
	case strategy.ROLL_STRATEGY_SINGLE:
		return Strategy_SINGLE, nil
	default:
		return -1, twirp.InternalError(fmt.Sprintf("invalid strategy %q", s))
	}
}

func convertStrategyChange(inp *strategy.StrategyChange) (*StrategyChange, error) {
	strat, err := convertStrategy(inp.Strategy)
	if err != nil {
		return nil, err
	}
	return &StrategyChange{
		Message:  inp.Message,
		RollerId: inp.Roller,
		Strategy: strat,
		Time:     timestamppb.New(inp.Time),
		User:     inp.User,
	}, nil
}

func convertRevision(inp *revision.Revision) *Revision {
	return &Revision{
		Description:   inp.Description,
		Display:       inp.Display,
		Id:            inp.Id,
		Time:          timestamppb.New(inp.Timestamp),
		Url:           inp.URL,
		InvalidReason: inp.InvalidReason,
	}
}

func convertRevisions(inp []*revision.Revision) []*Revision {
	rv := make([]*Revision, 0, len(inp))
	for _, v := range inp {
		rv = append(rv, convertRevision(v))
	}
	return rv
}

func convertConfig(inp *config.Config) *AutoRollConfig {
	var validModes []Mode
	if len(inp.ValidModes) > 0 {
		validModes = make([]Mode, 0, len(inp.ValidModes))
		for _, m := range inp.ValidModes {
			validModes = append(validModes, Mode(m))
		}
	}
	return &AutoRollConfig{
		ChildBugLink:        inp.ChildBugLink,
		ParentBugLink:       inp.ParentBugLink,
		ParentWaterfall:     inp.ParentWaterfall,
		RollerId:            inp.RollerName,
		SupportsManualRolls: inp.SupportsManualRolls,
		TimeWindow:          inp.TimeWindow,
		ValidModes:          validModes,
	}
}

func convertManualRollRequests(inp []*manual.ManualRollRequest) ([]*ManualRoll, error) {
	rv := make([]*ManualRoll, 0, len(inp))
	for _, v := range inp {
		req, err := convertManualRollRequest(v)
		if err != nil {
			return nil, err
		}
		rv = append(rv, req)
	}
	return rv, nil
}

func convertManualRollResult(s manual.ManualRollResult) (ManualRoll_Result, error) {
	switch manual.ManualRollResult(s) {
	case manual.RESULT_UNKNOWN:
		return ManualRoll_UNKNOWN, nil
	case manual.RESULT_FAILURE:
		return ManualRoll_FAILURE, nil
	case manual.RESULT_SUCCESS:
		return ManualRoll_SUCCESS, nil
	default:
		return -1, twirp.InternalError(fmt.Sprintf("invalid manual roll result %q", s))
	}
}

func convertManualRollStatus(s manual.ManualRollStatus) (ManualRoll_Status, error) {
	switch manual.ManualRollStatus(s) {
	case manual.STATUS_PENDING:
		return ManualRoll_PENDING, nil
	case manual.STATUS_STARTED:
		return ManualRoll_PENDING, nil
	case manual.STATUS_COMPLETE:
		return ManualRoll_COMPLETED, nil
	default:
		return -1, twirp.InternalError(fmt.Sprintf("invalid manual roll status %q", s))
	}
}

func convertManualRollRequest(inp *manual.ManualRollRequest) (*ManualRoll, error) {
	res, err := convertManualRollResult(inp.Result)
	if err != nil {
		return nil, err
	}
	st, err := convertManualRollStatus(inp.Status)
	if err != nil {
		return nil, err
	}
	return &ManualRoll{
		Id:                inp.Id,
		RollerId:          inp.RollerName,
		Revision:          inp.Revision,
		Requester:         inp.Requester,
		Result:            res,
		Status:            st,
		Timestamp:         timestamppb.New(inp.Timestamp),
		Url:               inp.Url,
		Canary:            inp.Canary,
		DryRun:            inp.DryRun,
		NoEmail:           inp.NoEmail,
		NoResolveRevision: inp.NoResolveRevision,
	}, nil
}

func convertStatus(st *status.AutoRollStatus, cfg *config.Config, modeChange *modes.ModeChange, strat *strategy.StrategyChange, manualReqs []*manual.ManualRollRequest, cleanupHistory []*roller_cleanup.CleanupRequest) (*AutoRollStatus, error) {
	mode := modes.ModeRunning
	if modeChange != nil {
		mode = modeChange.Mode
	}
	ms, err := convertMiniStatus(&st.AutoRollMiniStatus, cfg.RollerName, mode, cfg.ChildDisplayName, cfg.ParentDisplayName)
	if err != nil {
		return nil, err
	}
	var mc *ModeChange
	if modeChange != nil {
		mc, err = convertModeChange(modeChange)
		if err != nil {
			return nil, err
		}
	}
	var sc *StrategyChange
	if strat != nil {
		sc, err = convertStrategyChange(strat)
		if err != nil {
			return nil, err
		}
	}
	lastRoll, err := convertRollCL(st.LastRoll)
	if err != nil {
		return nil, err
	}
	currentRoll, err := convertRollCL(st.CurrentRoll)
	if err != nil {
		return nil, err
	}
	recentRolls, err := convertRollCLs(st.Recent)
	if err != nil {
		return nil, err
	}
	var manualRolls []*ManualRoll
	if manualReqs != nil {
		manualRolls, err = convertManualRollRequests(manualReqs)
		if err != nil {
			return nil, err
		}
	}
	var cleanupReq *CleanupRequest
	if len(cleanupHistory) != 0 && cleanupHistory[0].NeedsCleanup {
		cleanupReq = &CleanupRequest{
			NeedsCleanup:  cleanupHistory[0].NeedsCleanup,
			User:          cleanupHistory[0].User,
			Timestamp:     timestamppb.New(cleanupHistory[0].Timestamp),
			Justification: cleanupHistory[0].Justification,
		}
	}
	rv := &AutoRollStatus{
		CleanupRequested:   cleanupReq,
		MiniStatus:         ms,
		Status:             st.Status,
		Config:             convertConfig(cfg),
		FullHistoryUrl:     st.FullHistoryUrl,
		IssueUrlBase:       st.IssueUrlBase,
		Mode:               mc,
		Strategy:           sc,
		NotRolledRevisions: convertRevisions(st.NotRolledRevisions),
		CurrentRoll:        currentRoll,
		LastRoll:           lastRoll,
		RecentRolls:        recentRolls,
		ManualRolls:        manualRolls,
		Error:              st.Error,
		ThrottledUntil:     timestamppb.New(time.Unix(st.ThrottledUntil, 0)),
	}
	return rv, nil
}
