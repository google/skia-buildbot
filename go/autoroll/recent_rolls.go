package autoroll

import "sync"

const RECENT_ROLLS_LENGTH = 10

// RecentRolls is a struct used for storing and retrieving recent DEPS rolls.
type RecentRolls struct {
	db     *db
	recent []*AutoRollIssue
	mtx    sync.RWMutex
}

// newRecentRolls returns a new RecentRolls instance.
func newRecentRolls(d *db) (*RecentRolls, error) {
	recentRolls := &RecentRolls{
		db: d,
	}
	if err := recentRolls.refreshRecentRolls(); err != nil {
		return nil, err
	}
	return recentRolls, nil
}

// Add adds a DEPS roll to the recent rolls list.
func (r *RecentRolls) Add(roll *AutoRollIssue) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if err := r.db.InsertRoll(roll); err != nil {
		return err
	}
	return r.refreshRecentRolls()
}

// Update updates the given DEPS roll in the recent rolls list.
func (r *RecentRolls) Update(roll *AutoRollIssue) error {
	r.mtx.Lock()
	defer r.mtx.Unlock()
	if err := r.db.UpdateRoll(roll); err != nil {
		return err
	}
	return r.refreshRecentRolls()
}

// GetRecentRolls returns a copy of the recent rolls list.
func (r *RecentRolls) GetRecentRolls() []*AutoRollIssue {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	recent := make([]*AutoRollIssue, 0, len(r.recent))
	for _, r := range r.recent {
		recent = append(recent, &(*r))
	}
	return recent
}

// CurrentRoll returns a copy of the currently-active DEPS roll, or nil if none
// exists.
func (r *RecentRolls) CurrentRoll() *AutoRollIssue {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	if len(r.recent) == 0 {
		return nil
	}
	if r.recent[0].Closed {
		return nil
	}
	return &(*r.recent[0])
}

// LastRoll returns a copy of the last DEPS roll, if one exists, and nil
// otherwise.
func (r *RecentRolls) LastRoll() *AutoRollIssue {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	if len(r.recent) > 0 && r.recent[0].Closed {
		return r.recent[0]
	} else if len(r.recent) > 1 {
		return r.recent[1]
	}
	return nil
}

// refreshRecentRolls refreshes the list of recent DEPS rolls. Assumes the
// caller holds a write lock.
func (r *RecentRolls) refreshRecentRolls() error {
	// Load the last N rolls.
	recent, err := r.db.GetRecentRolls(RECENT_ROLLS_LENGTH)
	if err != nil {
		return err
	}
	r.recent = recent
	return nil
}
