package roller

import (
	"sync"

	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/autoroll/go/strategy"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/util"
)

// AutoRollStatus is a struct which provides roll-up status information about
// the AutoRoll Bot.
type AutoRollStatus struct {
	AutoRollMiniStatus
	ChildHead       string                    `json:"childHead"`
	CurrentRoll     *autoroll.AutoRollIssue   `json:"currentRoll"`
	Error           string                    `json:"error"`
	FullHistoryUrl  string                    `json:"fullHistoryUrl"`
	IssueUrlBase    string                    `json:"issueUrlBase"`
	LastRoll        *autoroll.AutoRollIssue   `json:"lastRoll"`
	LastRollRev     string                    `json:"lastRollRev"`
	Mode            *modes.ModeChange         `json:"mode"`
	Recent          []*autoroll.AutoRollIssue `json:"recent"`
	Status          string                    `json:"status"`
	Strategy        *strategy.StrategyChange  `json:"strategy"`
	ValidModes      []string                  `json:"validModes"`
	ValidStrategies []string                  `json:"validStrategies"`
}

// AutoRollMiniStatus is a struct which provides a minimal amount of status
// information about the AutoRoll Bot.
// TODO(borenet): Some of this duplicates things in AutoRollStatus. Revisit and
// either don't include AutoRollMiniStatus in AutoRollStatus or de-dupe the
// fields after revamping the UI.
type AutoRollMiniStatus struct {
	// Revision of the current roll, if any.
	CurrentRollRev string `json:"currentRollRev"`

	// Revision of the last successful roll.
	LastRollRev string `json:"lastRollRev"`

	// Current mode.
	Mode string `json:"mode"`

	// The number of failed rolls since the last successful roll.
	NumFailedRolls int `json:"numFailed"`

	// The number of commits which have not been rolled.
	NumNotRolledCommits int `json:"numBehind"`
}

// AutoRollStatusCache is a struct used for caching roll-up status
// information about the AutoRoll Bot.
type AutoRollStatusCache struct {
	currentRoll     *autoroll.AutoRollIssue
	fullHistoryUrl  string
	issueUrlBase    string
	lastError       string
	lastRoll        *autoroll.AutoRollIssue
	lastRollRev     string
	numFailed       int
	numNotRolled    int
	mode            *modes.ModeChange
	mtx             sync.RWMutex
	recent          []*autoroll.AutoRollIssue
	status          string
	strategy        *strategy.StrategyChange
	validStrategies []string
}

// Get returns the current status information.
func (c *AutoRollStatusCache) Get(includeError bool, cleanIssue func(*autoroll.AutoRollIssue)) *AutoRollStatus {
	if cleanIssue == nil {
		cleanIssue = func(*autoroll.AutoRollIssue) {}
	}
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	recent := make([]*autoroll.AutoRollIssue, 0, len(c.recent))
	for _, r := range c.recent {
		cpy := r.Copy()
		cleanIssue(cpy)
		recent = append(recent, cpy)
	}
	validModes := make([]string, len(modes.VALID_MODES))
	copy(validModes, modes.VALID_MODES)
	var mode *modes.ModeChange
	if c.mode != nil {
		mode = c.mode.Copy()
	}
	s := &AutoRollStatus{
		AutoRollMiniStatus: AutoRollMiniStatus{
			LastRollRev:         c.lastRollRev,
			NumFailedRolls:      c.numFailed,
			NumNotRolledCommits: c.numNotRolled,
		},
		FullHistoryUrl:  c.fullHistoryUrl,
		IssueUrlBase:    c.issueUrlBase,
		LastRollRev:     c.lastRollRev,
		Mode:            mode,
		Recent:          recent,
		Status:          c.status,
		Strategy:        c.strategy,
		ValidModes:      validModes,
		ValidStrategies: util.CopyStringSlice(c.validStrategies),
	}
	if c.currentRoll != nil {
		s.CurrentRoll = c.currentRoll.Copy()
		s.AutoRollMiniStatus.CurrentRollRev = c.currentRoll.RollingTo
		cleanIssue(s.CurrentRoll)
	}
	if c.lastRoll != nil {
		s.LastRoll = c.lastRoll.Copy()
		cleanIssue(s.LastRoll)
	}
	if c.mode != nil {
		s.Mode = c.mode.Copy()
		s.AutoRollMiniStatus.Mode = s.Mode.Mode
	}
	if includeError && c.lastError != "" {
		s.Error = c.lastError
	}
	return s
}

// Set sets the current status information.
func (c *AutoRollStatusCache) Set(s *AutoRollStatus) error {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	recent := make([]*autoroll.AutoRollIssue, 0, len(s.Recent))
	for _, r := range s.Recent {
		recent = append(recent, r.Copy())
	}
	c.currentRoll = nil
	if s.CurrentRoll != nil {
		c.currentRoll = s.CurrentRoll.Copy()
	}
	c.lastRoll = nil
	if s.LastRoll != nil {
		c.lastRoll = s.LastRoll.Copy()
	}
	c.fullHistoryUrl = s.FullHistoryUrl
	c.issueUrlBase = s.IssueUrlBase
	c.lastError = s.Error
	c.lastRollRev = s.LastRollRev
	c.mode = s.Mode.Copy()
	c.numFailed = s.NumFailedRolls
	c.numNotRolled = s.NumNotRolledCommits
	c.recent = recent
	c.status = s.Status
	c.strategy = s.Strategy
	c.validStrategies = util.CopyStringSlice(s.ValidStrategies)

	return nil
}

// GetMini returns minimal status information.
func (c *AutoRollStatusCache) GetMini() *AutoRollMiniStatus {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	rv := &AutoRollMiniStatus{
		LastRollRev:         c.lastRollRev,
		NumFailedRolls:      c.numFailed,
		NumNotRolledCommits: c.numNotRolled,
	}
	if c.currentRoll != nil {
		rv.CurrentRollRev = c.currentRoll.RollingTo
	}
	if c.mode != nil {
		rv.Mode = c.mode.Mode
	}
	return rv
}
