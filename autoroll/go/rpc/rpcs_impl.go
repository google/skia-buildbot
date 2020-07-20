package rpc

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/twitchtv/twirp"
	"go.skia.org/infra/autoroll/go/manual"
	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/revision"
	"go.skia.org/infra/autoroll/go/roller"
	"go.skia.org/infra/autoroll/go/status"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/autoroll/go/unthrottle"
	"go.skia.org/infra/go/allowed"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/login"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_opt=paths=source_relative --twirp_out=. --go_out=. ./rpc.proto
//go:generate mv ./go.skia.org/infra/autoroll/go/rpc/rpc.twirp.go ./rpc.twirp.go
//go:generate rm -rf ./go.skia.org
//go:generate goimports -w rpc.pb.go
//go:generate goimports -w rpc.twirp.go
//go:generate protoc --twirp_typescript_out=../../modules/rpc ./rpc.proto

var (
	// restrict* are used to restrict access to various endpoints.
	restrictAdmin = "Admin"
	restrictEdit  = "Edit"
	restrictView  = "View"
)

// NewAutoRollServer creates and returns a Twirp HTTP server.
func NewAutoRollServer(ctx context.Context, cfgs []*roller.AutoRollerConfig, manualRollDB manual.DB, viewers, editors, admins allowed.Allow) (http.Handler, error) {
	rollers := make(map[string]*autoroller, len(cfgs))
	for _, cfg := range cfgs {
		// Set up DBs for the roller.
		arbMode, err := modes.NewModeHistory(ctx, cfg.RollerName)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		go util.RepeatCtx(ctx, 10*time.Second, func(ctx context.Context) {
			if err := arbMode.Update(ctx); err != nil {
				sklog.Error(err)
			}
		})
		arbStatus, err := status.NewCache(ctx, cfg.RollerName)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		go util.RepeatCtx(ctx, 10*time.Second, func(ctx context.Context) {
			if err := arbStatus.Update(ctx); err != nil {
				sklog.Error(err)
			}
		})
		arbStrategy, err := strategy.NewStrategyHistory(ctx, cfg.RollerName, cfg.ValidStrategies())
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		go util.RepeatCtx(ctx, 10*time.Second, func(ctx context.Context) {
			if err := arbStrategy.Update(ctx); err != nil {
				sklog.Error(err)
			}
		})
		rollers[cfg.RollerName] = &autoroller{
			cfg:      cfg,
			mode:     arbMode,
			status:   arbStatus,
			strategy: arbStrategy,
		}
	}
	impl := &autoRollServerImpl{
		manualRollDB: manualRollDB,
		rollers:      rollers,
	}
	srv := NewAutoRollRPCsServer(impl, &twirp.ServerHooks{
		RequestRouted: func(ctx context.Context) (context.Context, error) {
			method, ok := twirp.MethodName(ctx)
			if !ok {
				return nil, twirp.InternalError("unable to find method name")
			}
			restrict := strings.SplitN(method, "_", 2)[0]
			var group allowed.Allow
			if restrict == restrictAdmin {
				group = admins
			} else if restrict == restrictEdit {
				group = editors
			} else if restrict == restrictView {
				group = viewers
			} else {
				err := fmt.Errorf("method name must begin with %s, %s, or %s", restrictAdmin, restrictEdit, restrictView)
				sklog.Error(err)
				return nil, twirp.InternalErrorWith(err)
			}
			if group != nil {
				email, err := getUser(ctx)
				if err != nil {
					return nil, err
				}
				if !group.Member(email) {
					return nil, twirp.NewError(twirp.PermissionDenied, "user does not have access")
				}
			}
			return ctx, nil
		},
	})
	return login.SessionMiddleware(srv), nil
}

// getUser is a helper function which obtains the logged-in user's email address
// or returns an error.
func getUser(ctx context.Context) (string, error) {
	session := login.GetSession(ctx)
	if session == nil {
		return "", twirp.NewError(twirp.Unauthenticated, "unable to find session")
	}
	return session.Email, nil
}

// autoRollServerImpl implements AutoRollRPCs.
type autoRollServerImpl struct {
	manualRollDB manual.DB
	rollers      map[string]*autoroller
}

// getRoller retrieves the given roller.
func (s *autoRollServerImpl) getRoller(roller string) (*autoroller, error) {
	rv, ok := s.rollers[roller]
	if !ok {
		return nil, twirp.NewError(twirp.NotFound, "Unknown roller")
	}
	return rv, nil
}

// View_GetRollers implements AutoRollRPCs.
func (s *autoRollServerImpl) View_GetRollers(ctx context.Context, req *GetRollersRequest) (*AutoRollMiniStatuses, error) {
	statuses := make([]*AutoRollMiniStatus, 0, len(s.rollers))
	for name, roller := range s.rollers {
		st := convertMiniStatus(roller.status.GetMini(), name, roller.mode.CurrentMode().Mode, roller.cfg.ChildDisplayName, roller.cfg.ParentDisplayName)
		statuses = append(statuses, st)
	}
	return &AutoRollMiniStatuses{
		Statuses: statuses,
	}, nil
}

// View_GetMiniStatus implements AutoRollRPCs.
func (s *autoRollServerImpl) View_GetMiniStatus(ctx context.Context, req *GetMiniStatusRequest) (*AutoRollMiniStatus, error) {
	roller, err := s.getRoller(req.Roller)
	if err != nil {
		return nil, err
	}
	return convertMiniStatus(roller.status.GetMini(), req.Roller, roller.mode.CurrentMode().Mode, roller.cfg.ChildDisplayName, roller.cfg.ParentDisplayName), nil
}

// View_GetStatus implements AutoRollRPCs.
func (s *autoRollServerImpl) View_GetStatus(ctx context.Context, req *GetStatusRequest) (*AutoRollStatus, error) {
	roller, err := s.getRoller(req.Roller)
	if err != nil {
		return nil, err
	}
	st := roller.status.Get()
	rv := &AutoRollStatus{
		MiniStatus:         convertMiniStatus(&st.AutoRollMiniStatus, req.Roller, roller.mode.CurrentMode().Mode, roller.cfg.ChildDisplayName, roller.cfg.ParentDisplayName),
		Status:             st.Status,
		Config:             convertConfig(roller.cfg),
		ChildHead:          st.ChildHead,
		FullHistoryUrl:     st.FullHistoryUrl,
		IssueUrlBase:       st.IssueUrlBase,
		Mode:               convertModeChange(roller.mode.CurrentMode()),
		Strategy:           convertStrategyChange(roller.strategy.CurrentStrategy()),
		NotRolledRevisions: convertRevisions(st.NotRolledRevisions),
		CurrentRoll:        convertRollCL(st.CurrentRoll),
		LastRoll:           convertRollCL(st.LastRoll),
		Recent:             convertRollCLs(st.Recent),
		ValidModes:         st.ValidModes,
		ValidStrategies:    st.ValidStrategies,
		ManualRequests:     nil, // Filled in below.
		Error:              st.Error,
		ThrottledUntil:     st.ThrottledUntil,
	}
	// Obtain manual roll requests, if supported by the roller.
	if roller.cfg.SupportsManualRolls {
		manualRequests, err := s.manualRollDB.GetRecent(roller.cfg.RollerName, len(rv.NotRolledRevisions))
		if err != nil {
			return nil, err
		}
		rv.ManualRequests = convertManualRollRequests(manualRequests)
	}
	return rv, nil
}

// Edit_SetMode implements AutoRollRPCs.
func (s *autoRollServerImpl) Edit_SetMode(ctx context.Context, req *SetModeRequest) (*AutoRollStatus, error) {
	user, err := getUser(ctx)
	if err != nil {
		return nil, err
	}
	roller, err := s.getRoller(req.Roller)
	if err != nil {
		return nil, err
	}
	if err := roller.mode.Add(ctx, req.Mode, user, req.Message); err != nil {
		return nil, err
	}
	return s.View_GetStatus(ctx, &GetStatusRequest{Roller: req.Roller})
}

// Edit_SetStrategy implements AutoRollRPCs.
func (s *autoRollServerImpl) Edit_SetStrategy(ctx context.Context, req *SetStrategyRequest) (*AutoRollStatus, error) {
	user, err := getUser(ctx)
	if err != nil {
		return nil, err
	}
	roller, err := s.getRoller(req.Roller)
	if err != nil {
		return nil, err
	}
	if err := roller.strategy.Add(ctx, req.Strategy, user, req.Message); err != nil {
		return nil, err
	}
	return s.View_GetStatus(ctx, &GetStatusRequest{Roller: req.Roller})
}

// Edit_CreateManualRoll implements AutoRollRPCs.
func (s *autoRollServerImpl) Edit_CreateManualRoll(ctx context.Context, req *CreateManualRollRequest) (*ManualRollRequest, error) {
	user, err := getUser(ctx)
	if err != nil {
		return nil, err
	}
	m := &manual.ManualRollRequest{
		RollerName: req.Roller,
		Revision:   req.Revision,
		Requester:  user,
	}
	m.Status = manual.STATUS_PENDING
	m.Timestamp = firestore.FixTimestamp(time.Now())
	if err := s.manualRollDB.Put(m); err != nil {
		return nil, err
	}
	return convertManualRollRequest(m), nil
}

// Edit_Unthrottle implements AutoRollRPCs.
func (s *autoRollServerImpl) Edit_Unthrottle(ctx context.Context, req *UnthrottleRequest) (*UnthrottleResponse, error) {
	if err := unthrottle.Unthrottle(ctx, req.Roller); err != nil {
		return nil, err
	}
	return &UnthrottleResponse{}, nil
}

// autoroller provides interactions with a single roller.
type autoroller struct {
	cfg *roller.AutoRollerConfig

	// Interactions with the roller through the DB.
	mode     *modes.ModeHistory
	status   *status.AutoRollStatusCache
	strategy *strategy.StrategyHistory
}

func convertMiniStatus(inp *status.AutoRollMiniStatus, roller, mode, childName, parentName string) *AutoRollMiniStatus {
	return &AutoRollMiniStatus{
		Roller:         roller,
		Mode:           mode,
		CurrentRollRev: inp.CurrentRollRev,
		LastRollRev:    inp.LastRollRev,
		ChildName:      childName,
		ParentName:     parentName,
		NumFailed:      int32(inp.NumFailedRolls),
		NumBehind:      int32(inp.NumNotRolledCommits),
	}
}

func convertRollCLs(inp []*autoroll.AutoRollIssue) []*AutoRollCL {
	rv := make([]*AutoRollCL, 0, len(inp))
	for _, v := range inp {
		rv = append(rv, convertRollCL(v))
	}
	return rv
}

func convertRollCL(inp *autoroll.AutoRollIssue) *AutoRollCL {
	if inp == nil {
		return nil
	}
	return &AutoRollCL{
		Id:          fmt.Sprintf("%d", inp.Issue),
		Result:      inp.Result,
		Subject:     inp.Subject,
		RollingTo:   inp.RollingTo,
		RollingFrom: inp.RollingFrom,
		Created:     inp.Created.Unix(),
		Modified:    inp.Modified.Unix(),
		TryResults:  convertTryResults(inp.TryResults),
	}
}

func convertTryResults(inp []*autoroll.TryResult) []*TryResult {
	if inp == nil {
		return nil
	}
	rv := make([]*TryResult, 0, len(inp))
	for _, v := range inp {
		rv = append(rv, convertTryResult(v))
	}
	return rv
}

func convertTryResult(inp *autoroll.TryResult) *TryResult {
	return &TryResult{
		Name:     inp.Builder,
		Status:   inp.Status,
		Result:   inp.Result,
		Url:      inp.Url,
		Category: inp.Category,
	}
}

func convertModeChange(inp *modes.ModeChange) *ModeChange {
	return &ModeChange{
		Roller: inp.Roller,
		Mode:   inp.Mode,
		User:   inp.User,
		Time:   inp.Time.Unix(),
	}
}

func convertStrategyChange(inp *strategy.StrategyChange) *StrategyChange {
	return &StrategyChange{
		Roller:   inp.Roller,
		Strategy: inp.Strategy,
		User:     inp.User,
		Time:     inp.Time.Unix(),
	}
}

func convertRevision(inp *revision.Revision) *Revision {
	return &Revision{
		Id:      inp.Id,
		Display: inp.Display,
	}
}

func convertRevisions(inp []*revision.Revision) []*Revision {
	rv := make([]*Revision, 0, len(inp))
	for _, v := range inp {
		rv = append(rv, convertRevision(v))
	}
	return rv
}

func convertConfig(inp *roller.AutoRollerConfig) *AutoRollConfig {
	return &AutoRollConfig{
		ParentWaterfall:     inp.ParentWaterfall,
		RollerName:          inp.RollerName,
		SupportsManualRolls: inp.SupportsManualRolls,
		TimeWindow:          inp.TimeWindow,
	}
}

func convertManualRollRequests(inp []*manual.ManualRollRequest) []*ManualRollRequest {
	rv := make([]*ManualRollRequest, 0, len(inp))
	for _, v := range inp {
		rv = append(rv, convertManualRollRequest(v))
	}
	return rv
}

func convertManualRollRequest(inp *manual.ManualRollRequest) *ManualRollRequest {
	return &ManualRollRequest{
		Id:        inp.Id,
		Roller:    inp.RollerName,
		Revision:  inp.Revision,
		Requester: inp.Requester,
		Status:    string(inp.Status),
		Timestamp: inp.Timestamp.Unix(),
		Url:       inp.Url,
	}
}
