package expstorage

import (
	"context"
	"time"

	"go.skia.org/infra/go/eventbus"
	"golang.org/x/sync/errgroup"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

const (
	// EV_TRYJOB_EXP_CHANGED is the event type that is fired when the expectations
	// for an issue change. It sends an instance of *TryjobExpChange.
	EV_TRYJOB_EXP_CHANGED = "tryjobstore:change"
)

type cloudExpStore struct {
	issueID  int64
	client   *datastore.Client
	eventBus eventbus.EventBus

	// Use different entities depending on whether this manages the master
	// or issue expectations.
	changeEntity        ds.Kind
	testDigestExpEntity ds.Kind
	summaryEntity       ds.Kind
}

type IssueExpStoreFactory func(issueID int64) ExpectationsStore

func NewCloudExpectationsStore(client *datastore.Client, eventBus eventbus.EventBus) (ExpectationsStore, IssueExpStoreFactory, error) {
	if client == nil {
		return nil, nil, sklog.FmtErrorf("Received nil for datastore client.")
	}

	store := &cloudExpStore{
		issueID:             -1,
		changeEntity:        ds.MASTER_EXP_CHANGE,
		testDigestExpEntity: ds.MASTER_TEST_DIGEST_EXP,
		summaryEntity:       ds.MASTER_EXP_SUMMARY,
		client:              client,
	}

	factory := func(issueID int64) ExpectationsStore {
		return &cloudExpStore{
			issueID:             issueID,
			changeEntity:        ds.TRYJOB_EXP_CHANGE,
			testDigestExpEntity: ds.TRYJOB_TEST_DIGEST_EXP,
			summaryEntity:       ds.ISSUE_EXP_SUMMARY,
			client:              client,
			eventBus:            eventBus,
		}
	}

	return store, factory, nil
}

// Get the current classifications for image digests. The keys of the
// expectations map are the test names.
func (c *cloudExpStore) Get() (exp *Expectations, err error) {
	currentExp, err := c.getCurrentExpectations(nil)
	if err != nil {
		return nil, err
	}

	return currentExp.toExpectations(), nil
}

// 	// Get all expectation changes and iterate over them updating the result.
// 	expChangeKeys, _, err := c.getExpChanges(-1, -1, true)
// 	if err != nil {
// 		return nil, err
// 	}

// 	testChanges := make([][]*TestDigestExp, len(expChangeKeys), len(expChangeKeys))
// 	// Iterate over the expectations build the expectations.
// 	var egroup errgroup.Group
// 	for idx, key := range expChangeKeys {
// 		func(idx int, key *datastore.Key) {
// 			egroup.Go(func() error {
// 				_, testChanges[idx], err = c.getTestDigestExps(key, false)
// 				return err
// 			})
// 		}(idx, key)
// 	}

// 	if err := egroup.Wait(); err != nil {
// 		return nil, err
// 	}

// 	ret := NewExpectations()
// 	for _, expByChange := range testChanges {
// 		if len(expByChange) > 0 {
// 			for _, oneChange := range expByChange {
// 				ret.SetTestExpectation(oneChange.Name, oneChange.Digest, types.LabelFromString(oneChange.Label))
// 			}
// 		}
// 	}
// 	return ret, nil
// }

// AddChange writes the given classified digests to the database and records the
// user that made the change.
func (c *cloudExpStore) AddChange(changes map[string]types.TestClassification, userId string) (err error) {
	// Write the change record.
	ctx := context.Background()
	expChange := &ExpChange{
		IssueID:   c.issueID,
		UserID:    userId,
		TimeStamp: util.TimeStamp(time.Millisecond),
	}

	var changeKey *datastore.Key
	if changeKey, err = c.client.Put(ctx, ds.NewKey(c.testDigestExpEntity), expChange); err != nil {
		return err
	}

	// If we have an error later make sure to delete change record.
	defer func() {
		if err != nil {
			go func() {
				if err := c.deleteExpChanges([]*datastore.Key{changeKey}); err != nil {
					sklog.Errorf("Error deleting expectation change %s: %s", changeKey.String(), err)
				}
			}()
		}
	}()

	tcKeys, testChanges := buildExpCollection(changes, c.testDigestExpEntity, changeKey)
	if _, err = c.client.PutMulti(ctx, tcKeys, testChanges); err != nil {
		return err
	}

	updateFn := func(tx *datastore.Transaction) error {
		// Start transaction to:
		//	- add the change to the summary.
		//  - store the latest entries to deal eventual consistency.
		//  - mark the change as valid.
		var egroup errgroup.Group

		// Update the recent changes so we get full consistency on queries.
		egroup.Go(func() error {
			matView, err := c.getRecentExpChanges(tx)
			if err != nil {
				return err
			}
			matView.Update(changeKey, expChange.TimeStamp)
			return c.putRecentExpChanges(tx, matView)
		})

		// Update the overall expectations.
		egroup.Go(func() error {
			var err error
			currentExp, err := c.getCurrentExpectations(tx)
			if err != nil {
				return err
			}
			currentExp.update(changes)
			return c.putCurrentExpectations(tx, currentExp)
		})

		if err := egroup.Wait(); err != nil {
			return err
		}

		// Mark the expectation change as valid.
		expChange.OK = true
		if _, err = tx.Put(changeKey, expChange); err != nil {
			return err
		}
		return nil
	}

	// Run the relevant updates in a transaction.
	if _, err = c.client.RunInTransaction(ctx, updateFn); err != nil {
		return err
	}

	if c.eventBus != nil {
		c.eventBus.Publish(EV_TRYJOB_EXP_CHANGED, expChange, false)
	}
	return nil
}

func (c *cloudExpStore) getRecentExpChanges(tx *datastore.Transaction) (*MatView, error) {
	ret := emptyMatView()
	err := tx.Get(c.getSummaryKey(), ret)
	if err == nil || err == datastore.ErrNoSuchEntity {
		return ret, nil
	}
	return nil, err
}

func (c *cloudExpStore) putRecentExpChanges(tx *datastore.Transaction, matView *MatView) error {
	_, err := tx.Put(c.getSummaryKey(), matView)
	return err
}

func (c *cloudExpStore) getCurrentExpectations(tx *datastore.Transaction) (ExpCollection, error) {
	_, ret, err := c.getTestDigestExps(tx, c.getSummaryKey(), false)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (c *cloudExpStore) putCurrentExpectations(tx *datastore.Transaction, currentExp ExpCollection) error {
	keys, err := currentExp.getKeys(c.summaryEntity, c.getSummaryKey())
	if err != nil {
		return err
	}
	if _, err := tx.PutMulti(keys, currentExp); err != nil {
		return err
	}
	return nil
}

// QueryLog allows to paginate through the changes in the expectations.
// If details is true the result will include a list of triage operations
// that were part a change.
func (c *cloudExpStore) QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error) {
	// TODO(stephana): Optimize this so we don't make the first request just to obtain the total.
	allKeys, _, err := c.getExpChanges(-1, -1, true)
	if err != nil {
		return nil, 0, sklog.FmtErrorf("Error retrieving keys for expectation changes: %s", err)
	}

	_, expChanges, err := c.getExpChanges(offset, size, false)
	if err != nil {
		return nil, 0, sklog.FmtErrorf("Error retrieving expectation changes: %s", err)
	}

	ret := make([]*TriageLogEntry, 0, len(expChanges))
	for _, change := range expChanges {
		ret = append(ret, &TriageLogEntry{
			ID:           int(change.ChangeID.ID),
			Name:         change.UserID,
			TS:           change.TimeStamp,
			ChangeCount:  int(change.Count),
			Details:      nil,
			UndoChangeID: int(change.UndoChangeID),
		})
	}

	return ret, len(allKeys), nil
}

// UndoChange reverts a change by setting all testname/digest pairs of the
// original change to the label they had before the change was applied.
// A new entry is added to the log with a reference to the change that was
// undone.
func (c *cloudExpStore) UndoChange(changeID int, userID string) (map[string]types.TestClassification, error) {
	return nil, nil
}

// removeChange removes the given digests from the expectations store.
// The key in changes is the test name which maps to a list of digests
// to remove. Used for testing only.
func (c *cloudExpStore) removeChange(changes map[string]types.TestClassification) error {
	return nil
}

// getExpChangesForIssue returns all the expectation changes for the given issue
// in revers chronological order. offset and size pick a subset of the result.
// Both are only considered if they are larger than 0. keysOnly indicates that we
// want keys only.
func (c *cloudExpStore) getExpChanges(offset, size int, keysOnly bool) ([]*datastore.Key, []*ExpChange, error) {
	q := ds.NewQuery(c.changeEntity).
		Filter("OK =", true).
		Order("TimeStamp")

	if c.issueID > 0 {
		q = q.Filter("IssueID =", c.issueID)
	}

	if keysOnly {
		q = q.KeysOnly()
	}

	if offset > 0 {
		q = q.Offset(offset)
	}

	if size > 0 {
		q = q.Limit(size)
	}

	var expChanges []*ExpChange
	keys, err := c.client.GetAll(context.Background(), q, &expChanges)
	return keys, expChanges, err
}

// getTestDigstExpectations gets all expectations for the given change.
func (c *cloudExpStore) getTestDigestExps(tx *datastore.Transaction, parentKey *datastore.Key, keysOnly bool) ([]*datastore.Key, ExpCollection, error) {
	q := ds.NewQuery(c.testDigestExpEntity).Ancestor(parentKey)

	if tx != nil {
		q = q.Transaction(tx)
	}

	if keysOnly {
		q = q.KeysOnly()
	}

	var exps ExpCollection
	expsKeys, err := c.client.GetAll(context.Background(), q, &exps)
	if err != nil {
		return nil, nil, err
	}
	return expsKeys, exps, nil
}

// deleteExpChanges deletes the given expectation changes.
func (c *cloudExpStore) deleteExpChanges(keys []*datastore.Key) error {
	return c.client.DeleteMulti(context.Background(), keys)
}

// getIssueKey returns a datastore key for the given issue id.
func (c *cloudExpStore) getSummaryKey() *datastore.Key {
	ret := ds.NewKey(c.summaryEntity)
	ret.ID = c.issueID
	return ret
}
