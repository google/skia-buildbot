package expstorage

import (
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/types"
)

// Wraps around an ExpectationsStore and caches the expectations using
// MemExpectationsStore.
type CachingExpectationStore struct {
	store    DEPRECATED_ExpectationsStore
	cache    DEPRECATED_ExpectationsStore
	eventBus eventbus.EventBus
	refresh  bool
}

func NewCachingExpectationStore(store DEPRECATED_ExpectationsStore, eventBus eventbus.EventBus) *CachingExpectationStore {
	ret := &CachingExpectationStore{
		store:    store,
		cache:    NewMemExpectationsStore(nil),
		eventBus: eventBus,
		refresh:  true,
	}

	// Prime the cache upon creation.
	// We ignore any error returned here for a simplified function signature and
	// also because any ExpectationsStore implementation (i.e. CloudExpectationsStore)
	// will do some connection checks before being passed to this instance.
	_, _ = ret.Get()

	// Register the events to update the cache.
	ret.eventBus.SubscribeAsync(EV_EXPSTORAGE_CHANGED, ret.addChangeToCache)

	return ret
}

// See ExpectationsStore interface.
func (c *CachingExpectationStore) Get() (exp types.Expectations, err error) {
	if c.refresh {
		c.refresh = false
		tempExp, err := c.store.Get()
		if err != nil {
			return nil, err
		}

		if err = c.cache.AddChange(tempExp.DeepCopy(), ""); err != nil {
			return nil, err
		}
	}
	return c.cache.Get()
}

// See ExpectationsStore interface.
func (c *CachingExpectationStore) AddChange(changedTests types.Expectations, userId string) error {
	if err := c.store.AddChange(changedTests, userId); err != nil {
		return err
	}
	// Fire an event that will trigger the addition to the cache and wait for it to complete.
	// This is necessary because events that change the cache could also come from the distributed
	// eventbus.
	waitCh := make(chan bool)
	c.eventBus.Publish(EV_EXPSTORAGE_CHANGED, evExpChange(changedTests, masterIssueID, waitCh), true)
	<-waitCh
	return nil
}

// addChangeToCache updates the cache and fires the change event.
func (c *CachingExpectationStore) addChangeToCache(evtChangedTests interface{}) {
	evtData := evtChangedTests.(*EventExpectationChange)
	changedTests := evtData.TestChanges

	// Split the changes into removal and addition.
	forRemoval := make(types.Expectations, len(changedTests))
	forAddition := make(types.Expectations, len(changedTests))
	for test, digests := range changedTests {
		for digest, label := range digests {
			if label == types.UNTRIAGED {
				if foundTest, ok := forRemoval[test]; ok {
					foundTest[digest] = label
				} else {
					forRemoval[test] = types.TestClassification{digest: label}
				}
			} else {
				if foundTest, ok := forAddition[test]; ok {
					foundTest[digest] = label
				} else {
					forAddition[test] = types.TestClassification{digest: label}
				}
			}
		}
	}

	if len(forRemoval) > 0 {
		if err := c.cache.RemoveChange(forRemoval); err != nil {
			sklog.Errorf("Error removing changed expectations to cache: %s", err)
		}
	}

	if len(forAddition) > 0 {
		if err := c.cache.AddChange(forAddition, ""); err != nil {
			sklog.Errorf("Error adding changed expectations to cache: %s", err)
		}
	}
	if evtData.waitCh != nil {
		evtData.waitCh <- true
	}
	sklog.Infof("Expectations change has been added to the cache.")
}

// RemoveChange implements the DEPRECATED_ExpectationsStore interface.
func (c *CachingExpectationStore) RemoveChange(changedDigests types.Expectations) error {
	if err := c.store.RemoveChange(changedDigests); err != nil {
		return err
	}

	// Fire an event that will trigger the addition to the cache.
	waitCh := make(chan bool)
	c.eventBus.Publish(EV_EXPSTORAGE_CHANGED, evExpChange(changedDigests, masterIssueID, waitCh), true)
	<-waitCh
	return nil
}

// See ExpectationsStore interface.
func (c *CachingExpectationStore) QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error) {
	return c.store.QueryLog(offset, size, details)
}

// See  ExpectationsStore interface.
func (c *CachingExpectationStore) UndoChange(changeID int64, userID string) (types.Expectations, error) {
	changedTests, err := c.store.UndoChange(changeID, userID)
	if err != nil {
		return nil, err
	}

	// Fire an event that will trigger the addition to the cache.
	waitCh := make(chan bool)
	c.eventBus.Publish(EV_EXPSTORAGE_CHANGED, evExpChange(changedTests, masterIssueID, waitCh), true)
	<-waitCh
	return changedTests, nil
}

// Clear implements the ExpectationsStore interface.
func (c *CachingExpectationStore) Clear() error {
	if err := c.store.Clear(); err != nil {
		return err
	}
	return c.cache.Clear()
}

// Make sure CachingExpectationStore fulfills the ExpectationsStore interface
var _ ExpectationsStore = (*CachingExpectationStore)(nil)

// Make sure CachingExpectationStore fulfills the DEPRECATED_ExpectationsStore interface
var _ DEPRECATED_ExpectationsStore = (*CachingExpectationStore)(nil)
