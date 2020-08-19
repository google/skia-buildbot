package strategy

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
	// StrategyHistoryLength is the number of StrategyChanges which may be
	// returned by StrategyHistory.GetHistory().
	StrategyHistoryLength = 25
)

// Fake ancestor we supply for all ModeChanges, to force consistency.
// We lose some performance this way but it keeps our tests from
// flaking.
func fakeAncestor() *datastore.Key {
	rv := ds.NewKey(ds.KIND_AUTOROLL_STRATEGY_ANCESTOR)
	rv.ID = 13 // Bogus ID.
	return rv
}

// StrategyChange is a struct used for describing a change in the AutoRoll strategy.
type StrategyChange struct {
	Message  string    `datastore:"message" json:"message"`
	Strategy string    `datastore:"strategy" json:"strategy"`
	Roller   string    `datastore:"roller" json:"-"`
	Time     time.Time `datastore:"time" json:"time"`
	User     string    `datastore:"user" json:"user"`
}

// Copy returns a copy of the StrategyChange.
func (c *StrategyChange) Copy() *StrategyChange {
	return &StrategyChange{
		Message:  c.Message,
		Strategy: c.Strategy,
		Roller:   c.Roller,
		Time:     c.Time,
		User:     c.User,
	}
}

// StrategyHistory tracks the history of StrategyChanges for all autorollers.
type StrategyHistory interface {
	// Add inserts a new StrategyChange.
	Add(ctx context.Context, s, user, message string) error
	// CurrentStrategy returns the current strategy, which is the most recently added
	// StrategyChange.
	CurrentStrategy() *StrategyChange
	// GetHistory returns a slice of the most recent StrategyChanges, most recent first.
	GetHistory() []*StrategyChange
	// Update refreshes the strategy history from the datastore.
	Update(ctx context.Context) error
}

// DatastoreStrategyHistory is a struct used for storing and retrieving strategy change history.
type DatastoreStrategyHistory struct {
	history         []*StrategyChange
	mtx             sync.RWMutex
	roller          string
	validStrategies []string
}

// NewDatastoreStrategyHistory returns a StrategyHistory instance.
func NewDatastoreStrategyHistory(ctx context.Context, roller string, validStrategies []string) (*DatastoreStrategyHistory, error) {
	sh := &DatastoreStrategyHistory{
		roller:          roller,
		validStrategies: validStrategies,
	}
	if err := sh.Update(ctx); err != nil {
		return nil, fmt.Errorf("Failed to refresh history: %s", err)
	}
	return sh, nil
}

// Add inserts a new StrategyChange.
func (sh *DatastoreStrategyHistory) Add(ctx context.Context, s, user, message string) error {
	if !util.In(s, sh.validStrategies) {
		return fmt.Errorf("Invalid strategy: %s; valid strategies: %v", s, sh.validStrategies)
	}
	strategyChange := &StrategyChange{
		Message:  message,
		Strategy: s,
		Roller:   sh.roller,
		Time:     time.Now(),
		User:     user,
	}
	if err := sh.put(ctx, strategyChange); err != nil {
		return err
	}
	return sh.Update(ctx)
}

// put inserts the StrategyChange into the datastore.
func (sh *DatastoreStrategyHistory) put(ctx context.Context, s *StrategyChange) error {
	key := ds.NewKey(ds.KIND_AUTOROLL_STRATEGY)
	key.Parent = fakeAncestor()
	_, err := ds.DS.RunInTransaction(ctx, func(tx *datastore.Transaction) error {
		_, err := tx.Put(key, s)
		return err
	})
	return err
}

// CurrentStrategy returns the current strategy, which is the most recently added
// StrategyChange.
func (sh *DatastoreStrategyHistory) CurrentStrategy() *StrategyChange {
	sh.mtx.RLock()
	defer sh.mtx.RUnlock()
	if len(sh.history) > 0 {
		return sh.history[0].Copy()
	}
	return nil
}

// GetHistory returns a slice of the most recent StrategyChanges, most recent first.
func (sh *DatastoreStrategyHistory) GetHistory() []*StrategyChange {
	sh.mtx.RLock()
	defer sh.mtx.RUnlock()
	rv := make([]*StrategyChange, 0, len(sh.history))
	for _, s := range sh.history {
		elem := new(StrategyChange)
		*elem = *s
		rv = append(rv, elem)
	}
	return rv
}

// getHistory retrieves recent strategy changes from the datastore.
func (sh *DatastoreStrategyHistory) getHistory(ctx context.Context) ([]*StrategyChange, error) {
	query := ds.NewQuery(ds.KIND_AUTOROLL_STRATEGY).Ancestor(fakeAncestor()).Filter("roller =", sh.roller).Order("-time").Limit(StrategyHistoryLength)
	var history []*StrategyChange
	if _, err := ds.DS.GetAll(ctx, query, &history); err != nil {
		return nil, fmt.Errorf("Failed to GetAll: %s", err)
	}
	return history, nil
}

// Update refreshes the strategy history from the datastore.
func (sh *DatastoreStrategyHistory) Update(ctx context.Context) error {
	history, err := sh.getHistory(ctx)
	if err != nil {
		return err
	}
	sh.mtx.Lock()
	defer sh.mtx.Unlock()
	sh.history = history
	return nil
}

var _ StrategyHistory = &DatastoreStrategyHistory{}
