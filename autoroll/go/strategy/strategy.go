package strategy

import (
	"fmt"
	"sync"
	"time"

	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	STRATEGY_HISTORY_LENGTH = 25
)

// StrategyChange is a struct used for describing a change in the AutoRoll strategy.
type StrategyChange struct {
	Message  string    `json:"message"`
	Strategy string    `json:"strategy"`
	Time     time.Time `json:"time"`
	User     string    `json:"user"`
}

// Copy returns a copy of the StrategyChange.
func (c *StrategyChange) Copy() *StrategyChange {
	return &StrategyChange{
		Message:  c.Message,
		Strategy: c.Strategy,
		Time:     c.Time,
		User:     c.User,
	}
}

// StrategyHistory is a struct used for storing and retrieving strategy change history.
type StrategyHistory struct {
	db              *db
	defaultStrategy string
	history         []*StrategyChange
	mtx             sync.RWMutex
	validStrategies []string
}

// NewStrategyHistory returns a StrategyHistory instance.
func NewStrategyHistory(dbFile, defaultStrategy string, validStrategies []string) (*StrategyHistory, error) {
	d, err := openDB(dbFile)
	if err != nil {
		return nil, err
	}
	sh := &StrategyHistory{
		db:              d,
		defaultStrategy: defaultStrategy,
		validStrategies: validStrategies,
	}
	if err := sh.refreshHistory(); err != nil {
		return nil, err
	}
	return sh, nil
}

// Close closes the database held by the StrategyHistory.
func (sh *StrategyHistory) Close() error {
	return sh.db.Close()
}

// Add inserts a new StrategyChange.
func (sh *StrategyHistory) Add(s, user, message string) error {
	sh.mtx.Lock()
	defer sh.mtx.Unlock()

	if !util.In(s, sh.validStrategies) {
		return fmt.Errorf("Invalid strategy: %s; valid strategies: %v", s, sh.validStrategies)
	}

	strategyChange := &StrategyChange{
		Message:  message,
		Strategy: s,
		Time:     time.Now(),
		User:     user,
	}

	if err := sh.db.SetStrategy(strategyChange); err != nil {
		return err
	}
	return sh.refreshHistory()
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
			Message: "Strategy history is empty!",
			Time:    time.Now(),
			User:    "autoroller",
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

// refreshHistory refreshes the strategy history from the database. Assumes that the
// caller holds a write lock.
func (sh *StrategyHistory) refreshHistory() error {
	history, err := sh.db.GetStrategyHistory(STRATEGY_HISTORY_LENGTH)
	if err != nil {
		return err
	}

	// If there's no history, set the initial strategy.
	if len(history) == 0 {
		if err := sh.db.SetStrategy(&StrategyChange{
			Message:  "Setting initial strategy.",
			Strategy: sh.defaultStrategy,
			Time:     time.Now(),
			User:     "AutoRoll Bot",
		}); err != nil {
			return err
		}
		history, err = sh.db.GetStrategyHistory(STRATEGY_HISTORY_LENGTH)
		if err != nil {
			return err
		}
	}

	sh.history = history
	return nil
}
