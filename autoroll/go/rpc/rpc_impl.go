package rpc

import (
	"context"
	"fmt"
	"net/http"
	"sort"
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
	"go.skia.org/infra/go/twirp_helper"
)

// Generate the go code from the protocol buffer definitions.
//go:generate protoc --go_opt=paths=source_relative --twirp_out=. --go_out=. ./rpc.proto
//go:generate mv ./go.skia.org/infra/autoroll/go/rpc/rpc.twirp.go ./rpc.twirp.go
//go:generate rm -rf ./go.skia.org
//go:generate goimports -w rpc.pb.go
//go:generate goimports -w rpc.twirp.go
//go:generate protoc --twirp_typescript_out=../../modules/rpc ./rpc.proto

// timeNowFunc allows tests to mock out time.Now() for testing.
var timeNowFunc = time.Now

// NewAutoRollServer creates and returns a Twirp HTTP server.
func NewAutoRollServer(ctx context.Context, rollers map[string]*AutoRoller, manualRollDB manual.DB, throttle unthrottle.Throttle, viewers, editors, admins allowed.Allow) http.Handler {
	impl := newAutoRollServerImpl(rollers, manualRollDB, throttle, viewers, editors, admins)
	srv := NewAutoRollRPCsServer(impl, nil)
	return twirp_helper.Middleware(srv)
}

// autoRollServerImpl implements AutoRollRPCs.
type autoRollServerImpl struct {
	*twirp_helper.AuthHelper
	manualRollDB manual.DB
	throttle     unthrottle.Throttle
	rollers      map[string]*AutoRoller
}

// newAutoRollServerImpl returns an autoRollServerImpl instance.
func newAutoRollServerImpl(rollers map[string]*AutoRoller, manualRollDB manual.DB, throttle unthrottle.Throttle, viewers, editors, admins allowed.Allow) *autoRollServerImpl {
	return &autoRollServerImpl{
		AuthHelper:   twirp_helper.NewAuthHelper(viewers, editors, admins),
		manualRollDB: manualRollDB,
		throttle:     throttle,
		rollers:      rollers,
	}
}

// getRoller retrieves the given roller.
func (s *autoRollServerImpl) getRoller(roller string) (*AutoRoller, error) {
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
	return s[a].Roller < s[b].Roller
}

func (s autoRollMiniStatusSlice) Swap(a, b int) {
	s[a], s[b] = s[b], s[a]
}

// GetRollers implements AutoRollRPCs.
func (s *autoRollServerImpl) GetRollers(ctx context.Context, req *GetRollersRequest) (*AutoRollMiniStatuses, error) {
	// Verify that the user has view access.
	if _, err := s.GetViewer(ctx); err != nil {
		return nil, err
	}
	statuses := make([]*AutoRollMiniStatus, 0, len(s.rollers))
	for name, roller := range s.rollers {
		st := convertMiniStatus(roller.Status.GetMini(), name, roller.Mode.CurrentMode().Mode, roller.Cfg.ChildDisplayName, roller.Cfg.ParentDisplayName)
		statuses = append(statuses, st)
	}
	// Sort for testing.
	sort.Sort(autoRollMiniStatusSlice(statuses))
	return &AutoRollMiniStatuses{
		Statuses: statuses,
	}, nil
}

// GetMiniStatus implements AutoRollRPCs.
func (s *autoRollServerImpl) GetMiniStatus(ctx context.Context, req *GetMiniStatusRequest) (*AutoRollMiniStatus, error) {
	// Verify that the user has view access.
	if _, err := s.GetViewer(ctx); err != nil {
		return nil, err
	}
	roller, err := s.getRoller(req.Roller)
	if err != nil {
		return nil, err
	}
	return convertMiniStatus(roller.Status.GetMini(), req.Roller, roller.Mode.CurrentMode().Mode, roller.Cfg.ChildDisplayName, roller.Cfg.ParentDisplayName), nil
}

// GetStatus implements AutoRollRPCs.
func (s *autoRollServerImpl) GetStatus(ctx context.Context, req *GetStatusRequest) (*AutoRollStatus, error) {
	// Verify that the user has view access.
	if _, err := s.GetViewer(ctx); err != nil {
		return nil, err
	}
	roller, err := s.getRoller(req.Roller)
	if err != nil {
		return nil, err
	}
	st := roller.Status.Get()
	var manualReqs []*manual.ManualRollRequest
	if roller.Cfg.SupportsManualRolls {
		manualReqs, err = s.manualRollDB.GetRecent(roller.Cfg.RollerName, len(st.NotRolledRevisions))
		if err != nil {
			return nil, err
		}
	}
	return convertStatus(st, roller.Cfg, roller.Mode.CurrentMode(), roller.Strategy.CurrentStrategy(), roller.Cfg.RollerName, roller.Cfg.ChildDisplayName, roller.Cfg.ParentDisplayName, manualReqs), nil
}

// SetMode implements AutoRollRPCs.
func (s *autoRollServerImpl) SetMode(ctx context.Context, req *SetModeRequest) (*AutoRollStatus, error) {
	// Verify that the user has edit access.
	user, err := s.GetEditor(ctx)
	if err != nil {
		return nil, err
	}
	roller, err := s.getRoller(req.Roller)
	if err != nil {
		return nil, err
	}
	if err := roller.Mode.Add(ctx, req.Mode, user, req.Message); err != nil {
		return nil, err
	}
	return s.GetStatus(ctx, &GetStatusRequest{Roller: req.Roller})
}

// SetStrategy implements AutoRollRPCs.
func (s *autoRollServerImpl) SetStrategy(ctx context.Context, req *SetStrategyRequest) (*AutoRollStatus, error) {
	// Verify that the user has edit access.
	user, err := s.GetEditor(ctx)
	if err != nil {
		return nil, err
	}
	roller, err := s.getRoller(req.Roller)
	if err != nil {
		return nil, err
	}
	if err := roller.Strategy.Add(ctx, req.Strategy, user, req.Message); err != nil {
		return nil, err
	}
	return s.GetStatus(ctx, &GetStatusRequest{Roller: req.Roller})
}

// CreateManualRoll implements AutoRollRPCs.
func (s *autoRollServerImpl) CreateManualRoll(ctx context.Context, req *CreateManualRollRequest) (*ManualRollRequest, error) {
	// Verify that the user has edit access.
	user, err := s.GetEditor(ctx)
	if err != nil {
		return nil, err
	}
	// Check that the roller exists.
	if _, err := s.getRoller(req.Roller); err != nil {
		return nil, err
	}
	m := &manual.ManualRollRequest{
		RollerName: req.Roller,
		Revision:   req.Revision,
		Requester:  user,
	}
	m.Status = manual.STATUS_PENDING
	m.Timestamp = firestore.FixTimestamp(timeNowFunc())
	if err := s.manualRollDB.Put(m); err != nil {
		return nil, err
	}
	return convertManualRollRequest(m), nil
}

// Unthrottle implements AutoRollRPCs.
func (s *autoRollServerImpl) Unthrottle(ctx context.Context, req *UnthrottleRequest) (*UnthrottleResponse, error) {
	// Verify that the user has edit access.
	if _, err := s.GetEditor(ctx); err != nil {
		return nil, err
	}
	// Check that the roller exists.
	if _, err := s.getRoller(req.Roller); err != nil {
		return nil, err
	}
	if err := s.throttle.Unthrottle(ctx, req.Roller); err != nil {
		return nil, err
	}
	return &UnthrottleResponse{}, nil
}

// AutoRoller provides interactions with a single roller.
type AutoRoller struct {
	Cfg *roller.AutoRollerConfig

	// Interactions with the roller through the DB.
	Mode     modes.ModeHistory
	Status   *status.Cache
	Strategy strategy.StrategyHistory
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

func convertStatus(st *status.AutoRollStatus, cfg *roller.AutoRollerConfig, mode *modes.ModeChange, strat *strategy.StrategyChange, roller, childName, parentName string, manualReqs []*manual.ManualRollRequest) *AutoRollStatus {
	rv := &AutoRollStatus{
		MiniStatus:         convertMiniStatus(&st.AutoRollMiniStatus, roller, mode.Mode, childName, parentName),
		Status:             st.Status,
		Config:             convertConfig(cfg),
		ChildHead:          st.ChildHead,
		FullHistoryUrl:     st.FullHistoryUrl,
		IssueUrlBase:       st.IssueUrlBase,
		Mode:               convertModeChange(mode),
		Strategy:           convertStrategyChange(strat),
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
	if manualReqs != nil {
		rv.ManualRequests = convertManualRollRequests(manualReqs)
	}
	return rv
}
