package autoroll_modes

import (
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	MODE_HISTORY_LENGTH = 25

	MODE_RUNNING = "running"
	MODE_STOPPED = "stopped"
	MODE_DRY_RUN = "dry run"
)

var (
	VALID_MODES = []string{
		MODE_RUNNING,
		MODE_STOPPED,
		MODE_DRY_RUN,
	}
)

// ModeChange is a struct used for describing a change in the AutoRoll mode.
type ModeChange struct {
	Message string    `json:"message"`
	Mode    string    `json:"mode"`
	Time    time.Time `json:"time"`
	User    string    `json:"user"`
}

// ModeHistory is a struct used for storing and retrieving mode change history.
type ModeHistory struct {
	db      *db
	history []*ModeChange
	mtx     sync.RWMutex
}

// NewModeHistory returns a ModeHistory instance.
func NewModeHistory(dbFile string) (*ModeHistory, error) {
	d, err := openDB(dbFile)
	if err != nil {
		return nil, err
	}
	mh := &ModeHistory{
		db: d,
	}
	if err := mh.refreshHistory(); err != nil {
		return nil, err
	}
	return mh, nil
}

// Close closes the database held by the ModeHistory.
func (mh *ModeHistory) Close() error {
	return mh.db.Close()
}

// Add inserts a new ModeChange.
func (mh *ModeHistory) Add(m, user, message string) error {
	if !util.In(m, VALID_MODES) {
		return fmt.Errorf("Invalid mode: %s", m)
	}

	modeChange := &ModeChange{
		Message: message,
		Mode:    m,
		Time:    time.Now(),
		User:    user,
	}

	mh.mtx.Lock()
	defer mh.mtx.Unlock()
	if err := mh.db.SetMode(modeChange); err != nil {
		return err
	}
	return mh.refreshHistory()
}

// CurrentMode returns the current mode, which is the most recently added
// ModeChange.
func (mh *ModeHistory) CurrentMode() string {
	mh.mtx.RLock()
	defer mh.mtx.RUnlock()
	if len(mh.history) > 0 {
		return mh.history[0].Mode
	} else {
		sklog.Errorf("Mode history is empty even after initialization!")
		return MODE_STOPPED
	}
}

// GetHistory returns a slice of the most recent ModeChanges, most recent first.
func (mh *ModeHistory) GetHistory() []*ModeChange {
	mh.mtx.RLock()
	defer mh.mtx.RUnlock()
	rv := make([]*ModeChange, 0, len(mh.history))
	for _, m := range mh.history {
		elem := new(ModeChange)
		*elem = *m
		rv = append(rv, elem)
	}
	return rv
}

// refreshHistory refreshes the mode history from the database. Assumes that the
// caller holds a write lock.
func (mh *ModeHistory) refreshHistory() error {
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
