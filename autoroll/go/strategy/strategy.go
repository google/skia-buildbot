package strategy

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
	STRATEGY_HISTORY_LENGTH = 25
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

// StrategyHistory is a struct used for storing and retrieving strategy change history.
type StrategyHistory struct {
	defaultStrategy string
	history         []*StrategyChange
	mtx             sync.RWMutex
	roller          string
	validStrategies []string
}

// NewStrategyHistory returns a StrategyHistory instance.
func NewStrategyHistory(ctx context.Context, roller, defaultStrategy string, validStrategies []string) (*StrategyHistory, error) {
	sh := &StrategyHistory{
		defaultStrategy: defaultStrategy,
		roller:          roller,
		validStrategies: validStrategies,
	}

	// Temporary measure to migrate internal rollers to new datastore
	// namespace: If there is no data, copy any existing data from the old
	// namespace to the new one.
	// TODO(borenet): Remove this after it has run once on all internal
	// rollers.
	data, err := sh.getHistory(ctx)
	if err != nil {
		return nil, err
	}
	if len(data) == 0 {
		sklog.Warningf("Migrating data to new namespace.")
		q := datastore.NewQuery(string(ds.KIND_AUTOROLL_STRATEGY)).Namespace(ds.AUTOROLL_NS).Filter("roller =", sh.roller)
		var data []*StrategyChange
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

	if err := sh.Update(ctx); err != nil {
		return nil, fmt.Errorf("Failed to refresh history: %s", err)
	}
	return sh, nil
}

// Add inserts a new StrategyChange.
func (sh *StrategyHistory) Add(ctx context.Context, s, user, message string) error {
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
func (sh *StrategyHistory) put(ctx context.Context, s *StrategyChange) error {
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
func (sh *StrategyHistory) CurrentStrategy() *StrategyChange {
	sh.mtx.RLock()
	defer sh.mtx.RUnlock()
	if len(sh.history) > 0 {
		return sh.history[0].Copy()
	} else {
		sklog.Errorf("Strategy history is empty even after initialization!")
		return &StrategyChange{
			Message:  "Strategy history is empty!",
			Roller:   sh.roller,
			Strategy: sh.defaultStrategy,
			Time:     time.Now(),
			User:     "autoroller",
		}
	}
}

// GetHistory returns a slice of the most recent StrategyChanges, most recent first.
func (sh *StrategyHistory) GetHistory() []*StrategyChange {
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
func (sh *StrategyHistory) getHistory(ctx context.Context) ([]*StrategyChange, error) {
	query := ds.NewQuery(ds.KIND_AUTOROLL_STRATEGY).Ancestor(fakeAncestor()).Filter("roller =", sh.roller).Order("-time").Limit(STRATEGY_HISTORY_LENGTH)
	var history []*StrategyChange
	if _, err := ds.DS.GetAll(ctx, query, &history); err != nil {
		return nil, fmt.Errorf("Failed to GetAll: %s", err)
	}
	return history, nil
}

// Update refreshes the strategy history from the datastore.
func (sh *StrategyHistory) Update(ctx context.Context) error {
	history, err := sh.getHistory(ctx)
	if err != nil {
		return err
	}

	// If there's no history, set the initial strategy.
	if len(history) == 0 {
		sklog.Info("Setting initial strategy.")
		if err := sh.put(ctx, &StrategyChange{
			Message:  "Setting initial strategy.",
			Strategy: sh.defaultStrategy,
			Roller:   sh.roller,
			Time:     time.Now(),
			User:     "AutoRoll Bot",
		}); err != nil {
			return err
		}
		history, err = sh.getHistory(ctx)
		if err != nil {
			return err
		}
	}

	sh.mtx.Lock()
	defer sh.mtx.Unlock()
	sh.history = history
	return nil
}
