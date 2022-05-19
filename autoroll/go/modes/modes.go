package modes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/util"
)

const (
	// ModeHistoryLength is the number of mode changes which may be returned
	// from ModeHistory.GetHistory().
	ModeHistoryLength = 25
)

// Valid autoroller modes.
const (
	ModeRunning = "running"
	ModeStopped = "stopped"
	ModeDryRun  = "dry run"
	ModeOffline = "offline"
)

var (
	// ValidModes lists the valid autoroller modes.
	ValidModes = []string{
		ModeRunning,
		ModeDryRun,
		ModeStopped,
		ModeOffline,
	}
)

// ModeHistory tracks the history of mode changes for the autoroller.
type ModeHistory interface {
	// Add a new ModeChange.
	Add(ctx context.Context, mode, user, message string) error
	// CurrentMode retrieves the most recent ModeChange.
	CurrentMode() *ModeChange
	// GetHistory returns a slice of recent ModeChanges. Its length is bounded
	// by ModeHistoryLength.
	GetHistory() []*ModeChange
	// Update the local view of the ModeChange history.
	Update(ctx context.Context) error
}

// Fake ancestor we supply for all ModeChanges, to force consistency.
// We lose some performance this way but it keeps our tests from
// flaking.
func fakeAncestor() *datastore.Key {
	rv := ds.NewKey(ds.KIND_AUTOROLL_MODE_ANCESTOR)
	rv.ID = 13 // Bogus ID.
	return rv
}

// ModeChange is a struct used for describing a change in the AutoRoll mode.
type ModeChange struct {
	Message string    `datastore:"message" json:"message"`
	Mode    string    `datastore:"mode" json:"mode"`
	Roller  string    `datastore:"roller" json:"-"`
	Time    time.Time `datastore:"time" json:"time"`
	User    string    `datastore:"user" json:"user"`
}

// Copy returns a copy of the ModeChange.
func (c *ModeChange) Copy() *ModeChange {
	return &ModeChange{
		Message: c.Message,
		Mode:    c.Mode,
		Roller:  c.Roller,
		Time:    c.Time,
		User:    c.User,
	}
}

// DatastoreModeHistory is a ModeHistory which uses Datastore.
type DatastoreModeHistory struct {
	history []*ModeChange
	mtx     sync.RWMutex
	roller  string
}

// NewDatastoreModeHistory returns a DatastoreModeHistory instance.
func NewDatastoreModeHistory(ctx context.Context, roller string) (*DatastoreModeHistory, error) {
	mh := &DatastoreModeHistory{
		roller: roller,
	}
	if err := mh.Update(ctx); err != nil {
		return nil, err
	}
	return mh, nil
}

// Add inserts a new ModeChange.
func (mh *DatastoreModeHistory) Add(ctx context.Context, mode, user, message string) error {
	if !util.In(mode, ValidModes) {
		return fmt.Errorf("Invalid mode: %s", mode)
	}
	modeChange := &ModeChange{
		Message: message,
		Mode:    mode,
		Roller:  mh.roller,
		Time:    time.Now(),
		User:    user,
	}
	if err := mh.put(ctx, modeChange); err != nil {
		return err
	}
	return mh.Update(ctx)
}

// put inserts the ModeChange into the datastore.
func (mh *DatastoreModeHistory) put(ctx context.Context, m *ModeChange) error {
	key := ds.NewKey(ds.KIND_AUTOROLL_MODE)
	key.Parent = fakeAncestor()
	_, err := ds.DS.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		_, err := tx.Put(key, m)
		return err
	})
	return err
}

// CurrentMode returns the current mode, which is the most recently added
// ModeChange.
func (mh *DatastoreModeHistory) CurrentMode() *ModeChange {
	mh.mtx.RLock()
	defer mh.mtx.RUnlock()
	if len(mh.history) > 0 {
		return mh.history[0].Copy()
	}
	return nil
}

// GetHistory returns a slice of the most recent ModeChanges, most recent first.
func (mh *DatastoreModeHistory) GetHistory() []*ModeChange {
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

// getHistory retrieves recent mode changes from the datastore.
func (mh *DatastoreModeHistory) getHistory(ctx context.Context) ([]*ModeChange, error) {
	query := ds.NewQuery(ds.KIND_AUTOROLL_MODE).Ancestor(fakeAncestor()).Filter("roller =", mh.roller).Order("-time").Limit(ModeHistoryLength)
	var history []*ModeChange
	if _, err := ds.DS.GetAll(ctx, query, &history); err != nil {
		return nil, err
	}
	return history, nil
}

// Update refreshes the mode history from the datastore.
func (mh *DatastoreModeHistory) Update(ctx context.Context) error {
	history, err := mh.getHistory(ctx)
	if err != nil {
		return err
	}
	mh.mtx.Lock()
	defer mh.mtx.Unlock()
	mh.history = history
	return nil
}

var _ ModeHistory = &DatastoreModeHistory{}
