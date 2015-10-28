package autoroll

import (
	"sync"
	"time"

	"github.com/skia-dev/glog"
)

const MODE_HISTORY_LENGTH = 25

// ModeChange is a struct used for describing a change in the AutoRoll mode.
type ModeChange struct {
	Message string    `json:"message"`
	Mode    Mode      `json:"mode"`
	Time    time.Time `json:"time"`
	User    string    `json:"user"`
}

// modeHistory is a struct used for storing and retrieving mode change history.
type modeHistory struct {
	db      *db
	history []*ModeChange
	mtx     sync.RWMutex
}

// newModeHistory returns a modeHistory instance.
func newModeHistory(d *db) (*modeHistory, error) {
	mh := &modeHistory{
		db: d,
	}
	if err := mh.refreshHistory(); err != nil {
		return nil, err
	}
	return mh, nil
}

// Add inserts a new ModeChange.
func (mh *modeHistory) Add(m *ModeChange) error {
	mh.mtx.Lock()
	defer mh.mtx.Unlock()
	if err := mh.db.SetMode(m); err != nil {
		return err
	}
	return mh.refreshHistory()
}

// CurrentMode returns the current mode, which is the most recently added
// ModeChange.
func (mh *modeHistory) CurrentMode() Mode {
	mh.mtx.RLock()
	defer mh.mtx.RUnlock()
	if len(mh.history) > 0 {
		return mh.history[0].Mode
	} else {
		glog.Errorf("Mode history is empty even after initialization!")
		return MODE_STOPPED
	}
}

// refreshHistory refreshes the mode history from the database. Assumes that the
// caller holds a write lock.
func (mh *modeHistory) refreshHistory() error {
	history, err := mh.db.GetModeHistory(MODE_HISTORY_LENGTH)
	if err != nil {
		return err
	}

	// If there's no history, set the initial mode.
	if len(history) == 0 {
		if err := mh.db.SetMode(&ModeChange{
			Message: "Setting initial mode.",
			Mode:    MODE_RUNNING,
			Time:    time.Now(),
			User:    "AutoRoll Bot",
		}); err != nil {
			return err
		}
		history, err = mh.db.GetModeHistory(MODE_HISTORY_LENGTH)
		if err != nil {
			return err
		}
	}

	mh.history = history
	return nil
}
