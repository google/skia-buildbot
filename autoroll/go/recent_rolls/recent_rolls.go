package recent_rolls

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"sync"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/autoroll"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"google.golang.org/api/iterator"
)

const (
	// Number of rolls to return from GetRecentRolls().
	RecentRollsLength = 10

	// loadRollsPageSize is the maximum number of rolls retrieved in a single
	// call to LoadRolls().
	loadRollsPageSize = 25
)

type DB interface {
	// Put inserts a roll into the RollsDB.
	Put(ctx context.Context, roller string, roll *autoroll.AutoRollIssue) error
	// Get retrieves a roll from the RollsDB.
	Get(ctx context.Context, roller string, issue int64) (*autoroll.AutoRollIssue, error)
	// GetRolls loads rolls from the database. Returns the rolls and a cursor
	// which may be used to retrieve more rolls.
	GetRolls(ctx context.Context, roller string, cursor string) ([]*autoroll.AutoRollIssue, string, error)
}

// Fake ancestor we supply for all ModeChanges, to force consistency.
// We lose some performance this way but it keeps our tests from
// flaking.
func fakeAncestor() *datastore.Key {
	rv := ds.NewKey(ds.KIND_AUTOROLL_ROLL_ANCESTOR)
	rv.ID = 13 // Bogus ID.
	return rv
}

// DsRoll is a struct used for storing autoroll.AutoRollIssue objects in
// datastore. The AutoRollIssue is gob-serialized before and after inserting
// to simplify the interactions with datastore.
type DsRoll struct {
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
	db                     DB
	lastSuccessfulRollTime time.Time
	numFailedrolls         int
	recent                 []*autoroll.AutoRollIssue
	roller                 string
	mtx                    sync.RWMutex
}

// NewRecentRolls returns a new RecentRolls instance.
func NewRecentRolls(ctx context.Context, db DB, roller string) (*RecentRolls, error) {
	recentRolls := &RecentRolls{
		roller: roller,
		db:     db,
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
	currentRoll := r.currentRoll()
	if currentRoll != nil {
		sklog.Warningf("There is already an active roll (%d), but another is being added (%d)", currentRoll.Issue, roll.Issue)
	}

	// Warn if the new roll is already closed.
	if roll.Closed {
		sklog.Warningf("Inserting a new roll which is already closed.")
	}

	if err := r.db.Put(ctx, r.roller, roll); err != nil {
		return err
	}
	return r.refreshRecentRolls(ctx)
}

// Update updates the given DEPS roll in the recent rolls list.
func (r *RecentRolls) Update(ctx context.Context, roll *autoroll.AutoRollIssue) error {
	// TODO(borenet): It would be better to pass in a function to perform
	// the desired modifications on the AutoRollIssue inside of the
	// transaction.
	if err := roll.Validate(); err != nil {
		return err
	}
	if err := r.db.Put(ctx, r.roller, roll); err != nil {
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
	return r.db.Get(ctx, r.roller, issue)
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

// LastSuccessfulRollTime returns the timestamp of the last successful roll.
func (r *RecentRolls) LastSuccessfulRollTime() time.Time {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.lastSuccessfulRollTime
}

// NumFailedRolls returns the number of failed rolls since the last successful
// roll.
func (r *RecentRolls) NumFailedRolls() int {
	r.mtx.RLock()
	defer r.mtx.RUnlock()
	return r.numFailedrolls
}

// refreshRecentRolls refreshes the list of recent DEPS rolls. Assumes the
// caller holds a write lock.
func (r *RecentRolls) refreshRecentRolls(ctx context.Context) error {
	// Load rolls until we have enough to satisfy RecentRollsLength and to
	// determine the number of failed rolls and timestamp of the last successful
	// roll.
	var history, rolls []*autoroll.AutoRollIssue
	var cursor string
	foundSuccessfulRoll := false
	lastSuccessfulRollTime := time.Time{}
	numFailedrolls := 0
	for {
		var err error
		rolls, cursor, err = r.db.GetRolls(ctx, r.roller, cursor)
		if err != nil {
			return err
		}
		history = append(history, rolls...)
		if !foundSuccessfulRoll {
			for _, roll := range rolls {
				if roll.Succeeded() {
					foundSuccessfulRoll = true
					lastSuccessfulRollTime = roll.Modified
					break
				} else if roll.Failed() {
					numFailedrolls++
				}
			}
		}
		if len(history) >= RecentRollsLength && foundSuccessfulRoll {
			break
		}
		if cursor == "" || len(rolls) == 0 {
			break
		}
	}
	historyLen := len(history)
	if historyLen > RecentRollsLength {
		historyLen = RecentRollsLength
	}

	r.mtx.Lock()
	defer r.mtx.Unlock()
	r.recent = history[:historyLen]
	r.lastSuccessfulRollTime = lastSuccessfulRollTime
	r.numFailedrolls = numFailedrolls
	return nil
}

// DatastoreRollsDB implements RollsDB using Datastore.
type DatastoreRollsDB struct{}

// NewDatastoreRollsDB returns a RollsDB instance which uses Datastore.
func NewDatastoreRollsDB(ctx context.Context) *DatastoreRollsDB {
	return &DatastoreRollsDB{}
}

// Get implements RollsDB.
func (d *DatastoreRollsDB) Get(ctx context.Context, roller string, issue int64) (*autoroll.AutoRollIssue, error) {
	query := ds.NewQuery(ds.KIND_AUTOROLL_ROLL).Ancestor(fakeAncestor()).Filter("rollerIssue =", fmt.Sprintf("%s_%d", roller, issue))
	var results []*DsRoll
	if _, err := ds.DS.GetAll(ctx, query, &results); err != nil {
		return nil, err
	}
	if len(results) == 0 {
		return nil, fmt.Errorf("Could not find issue %d", issue)
	} else if len(results) == 1 {
		var rv autoroll.AutoRollIssue
		if err := gob.NewDecoder(bytes.NewReader(results[0].Data)).Decode(&rv); err != nil {
			return nil, skerr.Wrap(err)
		}
		return &rv, nil
	} else {
		return nil, fmt.Errorf("Found more than one issue matching %d", issue)
	}
}

// Put implements RollsDB.
func (d *DatastoreRollsDB) Put(ctx context.Context, roller string, roll *autoroll.AutoRollIssue) error {
	var buf bytes.Buffer
	if err := gob.NewEncoder(&buf).Encode(roll); err != nil {
		return fmt.Errorf("Failed to encode roll: %s", err)
	}
	obj := &DsRoll{
		Data:          buf.Bytes(),
		Roller:        roller,
		RollerCreated: fmt.Sprintf("%s_%s", roller, roll.Created.UTC().Format(util.RFC3339NanoZeroPad)),
		RollerIssue:   fmt.Sprintf("%s_%d", roller, roll.Issue),
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

// LoadRolls implements RollsDB.
func (d *DatastoreRollsDB) GetRolls(ctx context.Context, roller, cursor string) ([]*autoroll.AutoRollIssue, string, error) {
	query := ds.NewQuery(ds.KIND_AUTOROLL_ROLL).Ancestor(fakeAncestor()).Filter("roller =", roller).Order("-rollerCreated").Limit(loadRollsPageSize)
	if cursor != "" {
		c, err := datastore.DecodeCursor(cursor)
		if err != nil {
			return nil, "", skerr.Wrap(err)
		}
		query = query.Start(c)
	}
	it := ds.DS.Run(ctx, query)
	rv := make([]*autoroll.AutoRollIssue, 0, loadRollsPageSize)
	var env DsRoll
	for i := 0; i < loadRollsPageSize; i++ {
		_, err := it.Next(&env)
		if err == iterator.Done {
			break
		} else if err != nil {
			return nil, "", skerr.Wrap(err)
		}
		roll := new(autoroll.AutoRollIssue)
		if err := gob.NewDecoder(bytes.NewReader(env.Data)).Decode(roll); err != nil {
			return nil, "", fmt.Errorf("Failed to decode roll: %s", err)
		}
		rv = append(rv, roll)
	}
	// Note: Unfortunately, datastore doesn't provide any indication that we've
	// reached the end of the results for a query, aside from returning fewer
	// results than the provided limit. This means that the client may have to
	// perform a call which returns zero results before it's clear that they've
	// retrieved all of the results.
	rvCursor := ""
	if len(rv) == loadRollsPageSize {
		c, err := it.Cursor()
		if err != nil {
			return nil, "", skerr.Wrap(err)
		}
		rvCursor = c.String()
	}
	return rv, rvCursor, nil
}

var _ DB = &DatastoreRollsDB{}
