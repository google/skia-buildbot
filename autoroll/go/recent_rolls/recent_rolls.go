package recent_rolls

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sync"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// Number of rolls to return from GetRecentRolls().
	RECENT_ROLLS_LENGTH = 10
)

// Fake ancestor we supply for all ModeChanges, to force consistency.
// We lose some performance this way but it keeps our tests from
// flaking.
func fakeAncestor() *datastore.Key {
	rv := ds.NewKey(ds.KIND_AUTOROLL_ROLL_ANCESTOR)
	rv.ID = 13 // Bogus ID.
	return rv
}

// dsRoll is a struct used for storing autoroll.AutoRollIssue objects in
// datastore. The AutoRollIssue is gob-serialized before and after inserting
// to simplify the interactions with datastore.
type dsRoll struct {
	// Data is the gob-serialized AutoRollIssue.
	Data []byte `datastore:"data,noindex"`

	// Name of the roller.
	Roller string `datastore:"roller"`

	// RollerCreated is synthesized from the roller name and the creation
	// time of the roll. This helps to keep the overall index well-
	// distributed.
	RollerCreated string `datastore:"rollerCreated"`

	// RollerIssue is synthesized from the roller name and the issue ID of
	// the roll. This helps to keep the overall index well-distributed. This
	// field is used as the ID in datastore.
	RollerIssue string `datastore:"rollerIssue"`
}

// RecentRolls is a struct used for storing and retrieving recent DEPS rolls.
type RecentRolls struct {
	recent []*autoroll.AutoRollIssue
	roller string
	mtx    sync.RWMutex
}

// NewRecentRolls returns a new RecentRolls instance.
func NewRecentRolls(ctx context.Context, roller string) (*RecentRolls, error) {
	recentRolls := &RecentRolls{
		roller: roller,
	}

	// Temporary measure to migrate internal rollers to new datastore
	// namespace: If there is no data, copy any existing data from the old
	// namespace to the new one.
	// TODO(borenet): Remove this after it has run once on all internal
	// rollers.
	data, err := recentRolls.getHistory(ctx)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		sklog.Warningf("Migrating data to new namespace.")
		q := datastore.NewQuery(string(ds.KIND_AUTOROLL_ROLL)).Namespace(ds.AUTOROLL_NS).Filter("roller =", recentRolls.roller)
		var data []*dsRoll
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

	if err := recentRolls.refreshRecentRolls(ctx); err != nil {
		return nil, err
	}
	return recentRolls, nil
}

// Add adds a DEPS roll to the recent rolls list.
func (r *RecentRolls) Add(ctx context.Context, roll *autoroll.AutoRollIssue) error {
	if err := roll.Validate(); err != nil {
		return err
	}

	// Warn if we already have an active roll.
	if r.currentRoll() != nil {
		sklog.Warningf("There is already an active roll, but another is being added!")
	}

	// Warn if the new roll is already closed.
	if roll.Closed {
		sklog.Warningf("Inserting a new roll which is already closed.")
	}

	if err := r.put(ctx, roll); err != nil {
		return err
	}
	return r.refreshRecentRolls(ctx)
}

// put inserts the roll into the datastore.
func (r *RecentRolls) put(ctx context.Context, roll *autoroll.AutoRollIssue) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(roll); err != nil {
		return fmt.Errorf("Failed to encode roll: %s", err)
	}
	obj := &dsRoll{
		Data:          buf.Bytes(),
		Roller:        r.roller,
		RollerCreated: fmt.Sprintf("%s_%s", r.roller, roll.Created.UTC().Format(util.RFC3339NanoZeroPad)),
		RollerIssue:   fmt.Sprintf("%s_%d", r.roller, roll.Issue),
	}
	key := ds.NewKey(ds.KIND_AUTOROLL_ROLL)
	key.Name = obj.RollerIssue
	key.Parent = fakeAncestor()
	_, err := ds.DS.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		_, err := tx.Put(key, obj)
		return err
	})
	if err != nil {
		return fmt.Errorf("Failed to insert roll: %s", err)
	}
	return nil
}

// Update updates the given DEPS roll in the recent rolls list.
func (r *RecentRolls) Update(ctx context.Context, roll *autoroll.AutoRollIssue) error {
	// TODO(borenet): It would be better to pass in a function to perform
	// the desired modifications on the AutoRollIssue inside of the
	// transaction.
	if err := roll.Validate(); err != nil {
		return err
	}
	if err := r.put(ctx, roll); err != nil {
		return err
	}
	return r.refreshRecentRolls(ctx)
}

// Get returns the given roll.
func (r *RecentRolls) Get(ctx context.Context, issue int64) (*autoroll.AutoRollIssue, error) {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	for _, roll := range r.recent {
		if roll.Issue == issue {
			return roll.Copy(), nil
		}
	}
	// Fall back to retrieving from datastore.
	query := ds.NewQuery(ds.KIND_AUTOROLL_ROLL).Ancestor(fakeAncestor()).Filter("rollerIssue =", fmt.Sprintf("%s_%d", r.roller, issue))
	var results []*autoroll.AutoRollIssue
	if _, err := ds.DS.GetAll(ctx, query, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("Could not find issue %d", issue)
	} else if len(results) == 1 {
		return results[0], nil
	} else {
		return nil, fmt.Errorf("Found more than one issue matching %d", issue)
	}
}

// GetRecentRolls returns a copy of the recent rolls list.
func (r *RecentRolls) GetRecentRolls() []*autoroll.AutoRollIssue {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	recent := make([]*autoroll.AutoRollIssue, 0, len(r.recent))
	for _, r := range r.recent {
		elem := new(autoroll.AutoRollIssue)
		*elem = *r
		recent = append(recent, elem)
	}
	return recent
}

// currentRoll returns the currently-active DEPS roll, or nil if none exists.
// Does not copy the roll. Expects that the caller holds a lock.
func (r *RecentRolls) currentRoll() *autoroll.AutoRollIssue {
	if len(r.recent) == 0 {
		return nil
	}
	if r.recent[0].Closed {
		return nil
	}
	return r.recent[0]
}

// CurrentRoll returns a copy of the currently-active DEPS roll, or nil if none
// exists.
func (r *RecentRolls) CurrentRoll() *autoroll.AutoRollIssue {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	current := r.currentRoll()
	if current != nil {
		rv := new(autoroll.AutoRollIssue)
		*rv = *current
		return rv
	}
	return nil
}

// LastRoll returns a copy of the last DEPS roll, if one exists, and nil
// otherwise.
func (r *RecentRolls) LastRoll() *autoroll.AutoRollIssue {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	if len(r.recent) > 0 && r.recent[0].Closed {
		rv := new(autoroll.AutoRollIssue)
		*rv = *r.recent[0]
		return rv
	} else if len(r.recent) > 1 {
		rv := new(autoroll.AutoRollIssue)
		*rv = *r.recent[1]
		return rv
	}
	return nil
}

func (r *RecentRolls) getHistory(ctx context.Context) ([]*autoroll.AutoRollIssue, error) {
	query := ds.NewQuery(ds.KIND_AUTOROLL_ROLL).Ancestor(fakeAncestor()).Filter("roller =", r.roller).Order("-rollerCreated").Limit(RECENT_ROLLS_LENGTH)
	var history []*dsRoll
	if _, err := ds.DS.GetAll(ctx, query, &history); err != nil {
		return nil, err
	}
	rv := make([]*autoroll.AutoRollIssue, 0, len(history))
	for _, enc := range history {
		roll := new(autoroll.AutoRollIssue)
		if err := gob.NewDecoder(bytes.NewReader(enc.Data)).Decode(&roll); err != nil {
			return nil, fmt.Errorf("Failed to decode roll: %s", err)
		}
		rv = append(rv, roll)
	}
	return rv, nil
}

// refreshRecentRolls refreshes the list of recent DEPS rolls. Assumes the
// caller holds a write lock.
func (r *RecentRolls) refreshRecentRolls(ctx context.Context) error {
	// Load the last N rolls.
	history, err := r.getHistory(ctx)
	if err != nil {
		return err
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.recent = history
	return nil
}
