package autorollerv2

import (
	"sync"

	"go.skia.org/infra/autoroll/go/autoroll_modes"
	"go.skia.org/infra/go/autoroll"
)

// AutoRollStatus is a struct which provides roll-up status information about
// the AutoRoll Bot.
type AutoRollStatus struct {
	CurrentRoll *autoroll.AutoRollIssue    `json:"currentRoll"`
	Error       string                     `json:"error"`
	GerritUrl   string                     `json:"gerritUrl"`
	LastRoll    *autoroll.AutoRollIssue    `json:"lastRoll"`
	LastRollRev string                     `json:"lastRollRev"`
	Mode        *autoroll_modes.ModeChange `json:"mode"`
	Recent      []*autoroll.AutoRollIssue  `json:"recent"`
	Status      string                     `json:"status"`
	ValidModes  []string                   `json:"validModes"`
}

// AutoRollStatusCache is a struct used for caching roll-up status
// information about the AutoRoll Bot.
type AutoRollStatusCache struct {
	currentRoll *autoroll.AutoRollIssue
	gerritUrl   string
	lastError   string
	lastRoll    *autoroll.AutoRollIssue
	lastRollRev string
	mode        *autoroll_modes.ModeChange
	mtx         sync.RWMutex
	recent      []*autoroll.AutoRollIssue
	status      string
}

// Get returns the current status information.
func (c *AutoRollStatusCache) Get(includeError bool) *AutoRollStatus {
	c.mtx.RLock()
	defer c.mtx.RUnlock()
	recent := make([]*autoroll.AutoRollIssue, 0, len(c.recent))
	for _, r := range c.recent {
		recent = append(recent, r.Copy())
	}
	validModes := make([]string, len(autoroll_modes.VALID_MODES))
	copy(validModes, autoroll_modes.VALID_MODES)
	s := &AutoRollStatus{
		GerritUrl:   c.gerritUrl,
		LastRollRev: c.lastRollRev,
		Mode:        c.mode.Copy(),
		Recent:      recent,
		Status:      c.status,
		ValidModes:  validModes,
	}
	if c.currentRoll != nil {
		s.CurrentRoll = c.currentRoll.Copy()
	}
	if c.lastRoll != nil {
		s.LastRoll = c.lastRoll.Copy()
	}
	if c.mode != nil {
		s.Mode = c.mode.Copy()
	}
	if includeError && c.lastError != "" {
		s.Error = c.lastError
	}
	return s
}

// set sets the current status information.
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
	c.recent = recent
	c.status = s.Status

	return nil
}
