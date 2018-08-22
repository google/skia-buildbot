package modes

import (
	"context"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
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

// ModeHistory is a struct used for storing and retrieving mode change history.
type ModeHistory struct {
	history []*ModeChange
	mtx     sync.RWMutex
	roller  string
}

// NewModeHistory returns a ModeHistory instance.
func NewModeHistory(ctx context.Context, roller string) (*ModeHistory, error) {
	mh := &ModeHistory{
		roller: roller,
	}

	// Temporary measure to migrate internal rollers to new datastore
	// namespace: If there is no data, copy any existing data from the old
	// namespace to the new one.
	// TODO(borenet): Remove this after it has run once on all internal
	// rollers.
	data, err := mh.getHistory(ctx)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		sklog.Warningf("Migrating data to new namespace.")
		q := datastore.NewQuery(string(ds.KIND_AUTOROLL_MODE)).Namespace(ds.AUTOROLL_NS).Filter("roller =", mh.roller)
		var data []*ModeChange
		keys, err := ds.DS.GetAll(ctx, q, &data)
		if err != nil {
			return nil, fmt.Errorf("Failed to retrieve old data: %s", err)
		}
		for _, key := range keys {
			key.Namespace = ds.AUTOROLL_INTERNAL_NS
			key.Parent.Namespace = ds.AUTOROLL_INTERNAL_NS
		}
		if err := util.ChunkIter(len(keys), 500 /* Maximum number allowed to insert into DS */, func(start, end int) error {
			_, err := ds.DS.PutMulti(ctx, keys[start:end], data[start:end])
			return err
		}); err != nil {
			return nil, fmt.Errorf("Failed to insert old data: %s", err)
		}
		sklog.Warningf("Finished migrating data to new namespace.")
	}

	if err := mh.Update(ctx); err != nil {
		return nil, err
	}
	return mh, nil
}

// Add inserts a new ModeChange.
func (mh *ModeHistory) Add(ctx context.Context, mode, user, message string) error {
	if !util.In(mode, VALID_MODES) {
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
func (mh *ModeHistory) put(ctx context.Context, m *ModeChange) error {
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
func (mh *ModeHistory) CurrentMode() *ModeChange {
	mh.mtx.RLock()
	defer mh.mtx.RUnlock()
	if len(mh.history) > 0 {
		return mh.history[0].Copy()
	} else {
		sklog.Errorf("Mode history is empty even after initialization!")
		return &ModeChange{
			Message: "Mode history is empty!",
			Mode:    MODE_STOPPED,
			Roller:  mh.roller,
			Time:    time.Now(),
			User:    "autoroller",
		}
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

// getHistory retrieves recent mode changes from the datastore.
func (mh *ModeHistory) getHistory(ctx context.Context) ([]*ModeChange, error) {
	query := ds.NewQuery(ds.KIND_AUTOROLL_MODE).Ancestor(fakeAncestor()).Filter("roller =", mh.roller).Order("-time").Limit(MODE_HISTORY_LENGTH)
	var history []*ModeChange
	if _, err := ds.DS.GetAll(ctx, query, &history); err != nil {
		return nil, err
	}
	return history, nil
}

// Update refreshes the mode history from the datastore.
func (mh *ModeHistory) Update(ctx context.Context) error {
	history, err := mh.getHistory(ctx)
	if err != nil {
		return err
	}

	// If there's no history, set the initial mode.
	if len(history) == 0 {
		sklog.Info("Setting initial mode.")
		if err := mh.put(ctx, &ModeChange{
			Message: "Setting initial mode.",
			Mode:    MODE_RUNNING,
			Roller:  mh.roller,
			Time:    time.Now(),
			User:    "AutoRoll Bot",
		}); err != nil {
			return err
		}
		history, err = mh.getHistory(ctx)
		if err != nil {
			return err
		}
	}

	mh.mtx.Lock()
	defer mh.mtx.Unlock()
	mh.history = history
	return nil
}
