package roller

import (
	"sync"

	"go.skia.org/infra/autoroll/go/modes"
	"go.skia.org/infra/go/autoroll"
)

// AutoRollStatus is a struct which provides roll-up status information about
// the AutoRoll Bot.
type AutoRollStatus struct {
	AutoRollMiniStatus
	ChildHead   string                    `json:"childHead"`
	CurrentRoll *autoroll.AutoRollIssue   `json:"currentRoll"`
	Error       string                    `json:"error"`
	GerritUrl   string                    `json:"gerritUrl"`
	LastRoll    *autoroll.AutoRollIssue   `json:"lastRoll"`
	LastRollRev string                    `json:"lastRollRev"`
	Mode        *modes.ModeChange         `json:"mode"`
	Recent      []*autoroll.AutoRollIssue `json:"recent"`
	Status      string                    `json:"status"`
	ValidModes  []string                  `json:"validModes"`
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
	currentRoll  *autoroll.AutoRollIssue
	gerritUrl    string
	lastError    string
	lastRoll     *autoroll.AutoRollIssue
	lastRollRev  string
	numFailed    int
	numNotRolled int
	mode         *modes.ModeChange
	mtx          sync.RWMutex
	recent       []*autoroll.AutoRollIssue
	status       string
}

// Get returns the current status information.
func (c *AutoRollStatusCache) Get(includeError bool) *AutoRollStatus {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	recent := make([]*autoroll.AutoRollIssue, 0, len(c.recent))
	for _, r := range c.recent {
		recent = append(recent, r.Copy())
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
		GerritUrl:   c.gerritUrl,
		LastRollRev: c.lastRollRev,
		Mode:        mode,
		Recent:      recent,
		Status:      c.status,
		ValidModes:  validModes,
	}
	if c.currentRoll != nil {
		s.CurrentRoll = c.currentRoll.Copy()
		s.AutoRollMiniStatus.CurrentRollRev = c.currentRoll.RollingTo
	}
	if c.lastRoll != nil {
		s.LastRoll = c.lastRoll.Copy()
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
	c.gerritUrl = s.GerritUrl
	c.lastRollRev = s.LastRollRev
	c.mode = s.Mode.Copy()
	c.numFailed = s.NumFailedRolls
	c.numNotRolled = s.NumNotRolledCommits
	c.recent = recent
	c.status = s.Status

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
