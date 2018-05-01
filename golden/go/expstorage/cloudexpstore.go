package expstorage

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"cloud.google.com/go/datastore"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/dsutil"
	"go.skia.org/infra/golden/go/types"
)

const (
	// EV_TRYJOB_EXP_CHANGED is the event type that is fired when the expectations
	// for an issue change. It sends an instance of *TryjobExpChange.
	EV_TRYJOB_EXP_CHANGED = "expstorage:tryjob-exp-change"
)

// cloudExpStore implements the ExpectationsStore interface with a the
// Google Cloud Datastore as its backend.
// Since the different between storing expectations for the master branch and
// the tryjobs of a Gerrit CL is only the includion of an issue id, it also
// simultaneously supports storing expectations for that Gerrit issue using
// the same interface and based on the same storage client.
type cloudExpStore struct {
	issueID    int64
	client     *datastore.Client
	eventBus   eventbus.EventBus
	listHelper *dsutil.ListHelper
	summaryKey *datastore.Key

	// Use different entities depending on whether this manages the master
	// or issue expectations.
	changeEntity        ds.Kind
	testDigestExpEntity ds.Kind
	eventExpChange      string
	globalEvent         bool
}

// IssueExpStoreFactory creates an ExpectationsStore instance for the given issue id.
type IssueExpStoreFactory func(issueID int64) ExpectationsStore

// NewCloudExpectationsStore returns an ExpectationsStore implementation based on
// Cloud Datastore for the master branch and a factory to create ExpectationsStore
// instances for Gerrit issues. The factory uses the same datastore client as the
// master store.
func NewCloudExpectationsStore(client *datastore.Client, eventBus eventbus.EventBus) (ExpectationsStore, IssueExpStoreFactory, error) {
	if client == nil {
		return nil, nil, sklog.FmtErrorf("Received nil for datastore client.")
	}

	// Create the instance for the master and set the target entities to for the
	// master branch.
	summaryKey := ds.NewKey(ds.RECENT_KEYS)
	summaryKey.Name = "expstorage-master"
	store := &cloudExpStore{
		issueID:             -1,
		changeEntity:        ds.MASTER_EXP_CHANGE,
		testDigestExpEntity: ds.MASTER_TEST_DIGEST_EXP,
		eventExpChange:      EV_EXPSTORAGE_CHANGED,
		globalEvent:         true,
		client:              client,
		eventBus:            eventBus,
		summaryKey:          summaryKey,
		listHelper:          dsutil.NewListHelper(client, summaryKey, dsutil.DefaultConsistencyDelta),
	}

	// The factory allows to create an isolated ExpectationStore instance for the
	// given issue.
	factory := func(issueID int64) ExpectationsStore {
		summaryKey := ds.NewKey(ds.RECENT_KEYS)
		summaryKey.Name = "expstorage-issue-" + strconv.FormatInt(issueID, 10)
		return &cloudExpStore{
			issueID:             issueID,
			changeEntity:        ds.TRYJOB_EXP_CHANGE,
			testDigestExpEntity: ds.TRYJOB_TEST_DIGEST_EXP,
			eventExpChange:      EV_TRYJOB_EXP_CHANGED,
			globalEvent:         false,
			client:              client,
			eventBus:            eventBus,
			summaryKey:          summaryKey,
			listHelper:          dsutil.NewListHelper(client, summaryKey, dsutil.DefaultConsistencyDelta),
		}
	}

	return store, factory, nil
}

// Get implements the ExpectationsStore interface.
func (c *cloudExpStore) Get() (exp *Expectations, err error) {
	currentExp, err := c.getCurrentExpectations(nil)
	if err != nil {
		return nil, err
	}

	return currentExp.toExpectations(true), nil
}

// AddChange implements the ExpectationsStore interface.
func (c *cloudExpStore) AddChange(changes map[string]types.TestClassification, userId string) error {
	return c.makeChange(changes, userId, 0)
}

// QueryLog implements the ExpectationsStore interface.
func (c *cloudExpStore) QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error) {
	allKeys, err := c.getExpChangeKeys(0)
	if err != nil {
		return nil, 0, sklog.FmtErrorf("Error retrieving keys for expectation changes: %s", err)
	}

	if offset < 0 {
		offset = 0
	}

	if size <= 0 {
		size = len(allKeys)
	}

	start := util.MinInt(offset, len(allKeys))
	end := util.MinInt(start+size, len(allKeys))
	retKeys := allKeys[start:end]

	ret := make([]*TriageLogEntry, 0, len(retKeys))
	var detailRecs [][]*TriageDetail = nil
	var egroup errgroup.Group
	egroup.Go(func() error {
		expChanges := make([]*ExpChange, len(retKeys))
		if err := c.client.GetMulti(context.Background(), retKeys, expChanges); err != nil {
			return sklog.FmtErrorf("Error retrieving expectation changes: %s", err)
		}

		for _, change := range expChanges {
			ret = append(ret, &TriageLogEntry{
				ID:           change.ChangeID.ID,
				Name:         change.UserID,
				TS:           change.TimeStamp,
				ChangeCount:  int(change.Count),
				Details:      nil,
				UndoChangeID: change.UndoChangeID,
			})
		}
		return nil
	})

	// If we want details fetch them in parallel to the change records.
	if details {
		detailRecs = make([][]*TriageDetail, len(retKeys))
		for idx, parentKey := range retKeys {
			func(idx int, parentKey *datastore.Key) {
				egroup.Go(func() error {
					_, expColl, err := c.getTestDigestExps(nil, parentKey)
					if err != nil {
						return sklog.FmtErrorf("Error retrieving change details: %s", err)
					}

					triageDetails := make([]*TriageDetail, 0, len(expColl[0].Names))
					for _, batch := range expColl {
						for nameIdx, name := range batch.Names {
							triageDetails = append(triageDetails, &TriageDetail{
								TestName: name,
								Digest:   batch.Digests[nameIdx],
								Label:    batch.Labels[nameIdx],
							})
						}
					}

					sort.Slice(triageDetails, func(i, j int) bool {
						return ((triageDetails[i].TestName < triageDetails[j].TestName) ||
							(triageDetails[i].Digest < triageDetails[j].Digest))
					})

					detailRecs[idx] = triageDetails
					return nil
				})
			}(idx, parentKey)
		}
	}

	// Wait for all queries to finish.
	if err := egroup.Wait(); err != nil {
		return nil, 0, err
	}

	// Fill in the details.
	if details {
		for idx, triageEntry := range ret {
			triageEntry.Details = detailRecs[idx]
		}
	}

	return ret, len(allKeys), nil
}

// UndoChange implements the ExpectationsStore interface.
func (c *cloudExpStore) UndoChange(changeID int64, userID string) (map[string]types.TestClassification, error) {
	// Make sure the entity is valid.
	if changeID <= 0 {
		return nil, sklog.FmtErrorf("Change with id %d does not exist.", changeID)
	}

	// Fetch the change and it's details.
	expChange := &ExpChange{}
	expChangeKey := ds.NewKey(c.changeEntity)
	expChangeKey.ID = changeID
	var egroup errgroup.Group
	egroup.Go(func() error {
		if err := c.client.Get(context.Background(), expChangeKey, expChange); err != nil {
			if err == datastore.ErrNoSuchEntity {
				return sklog.FmtErrorf("Change with id %d does not exist.", changeID)
			}
			return err
		}
		return nil
	})

	var changes map[string]types.TestClassification = nil
	egroup.Go(func() error {
		var err error
		var coll ExpCollection
		if _, coll, err = c.getTestDigestExps(nil, expChangeKey); err != nil {
			return sklog.FmtErrorf("Error retrieving expectation changes for change %d", changeID)
		}
		changes = coll.toExpectations(false).Tests
		for _, digests := range changes {
			for d := range digests {
				digests[d] = types.UNTRIAGED
			}
		}
		return nil
	})

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	// If this has been undone already, then don't do it.
	if expChange.UndoChangeID != 0 {
		return nil, fmt.Errorf("Unable to undo change %d which was created as an undo of change %d.", changeID, expChange.UndoChangeID)
	}

	allKeys, err := c.getExpChangeKeys(changeID)
	if err != nil {
		return nil, sklog.FmtErrorf("Error retrieving keys for expectation changes: %s", err)
	}

	details := make([]ExpCollection, len(allKeys))
	for idx, parentKey := range allKeys {
		func(idx int, parentKey *datastore.Key) {
			egroup.Go(func() error {
				var err error
				_, details[idx], err = c.getTestDigestExps(nil, parentKey)
				return err
			})
		}(idx, parentKey)
	}

	if err := egroup.Wait(); err != nil {
		return nil, sklog.FmtErrorf("Error retrieving change details: %s", err)
	}

	// Build the change that we need to make for the undo.
	for idx := len(details) - 1; idx >= 0; idx-- {
		for _, batch := range details[idx] {
			for idx, name := range batch.Names {
				digest := batch.Digests[idx]
				if _, ok := changes[name][digest]; ok {
					changes[name][digest] = types.LabelFromString(batch.Labels[idx])
				}
			}
		}
	}
	return changes, c.makeChange(changes, userID, changeID)
}

// TODO(stephana): The removeChange function is obsolete and should be removed.
// It was used for testing before the UndoChange function was added. It is simply
// wrong to change the expectations independently of a change record being added.

// removeChange implements the ExpectationsStore interface.
func (c *cloudExpStore) removeChange(changes map[string]types.TestClassification) error {
	for _, digests := range changes {
		for digest := range digests {
			digests[digest] = types.UNTRIAGED
		}
	}

	updateFn := func(tx *datastore.Transaction) error {
		return c.updateExpectations(tx, changes)
	}

	// Run the removal changes in a transaction.
	if _, err := c.client.RunInTransaction(context.Background(), updateFn); err != nil {
		return err
	}

	if c.eventBus != nil {
		// This is always a local event since it's only used for testing.
		c.eventBus.Publish(c.eventExpChange, evExpChange(changes, c.issueID), false)
	}
	return nil
}

// makeChange updates the expectations by adding a new change record to the datastore.
// If undoChangeId is larger than 0 then it will be recorded in the change record
// since this is an undo of an earlier change.
func (c *cloudExpStore) makeChange(changes map[string]types.TestClassification, userId string, undoChangeID int64) (err error) {
	// Write the change record.
	ctx := context.Background()
	timeStampMs := util.TimeStamp(time.Millisecond)
	expChange := &ExpChange{
		IssueID:      c.issueID,
		UserID:       userId,
		UndoChangeID: undoChangeID,
		TimeStamp:    timeStampMs,
	}

	changeKey := dsutil.TimeSortableKey(c.changeEntity, timeStampMs)
	if _, err = c.client.Put(ctx, changeKey, expChange); err != nil {
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

	// Write the changed expectations to the database.
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
		egroup.Go(func() error { return c.listHelper.Add(tx, changeKey) })

		// Update the overall expectations.
		egroup.Go(func() error { return c.updateExpectations(tx, changes) })

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
		c.eventBus.Publish(c.eventExpChange, evExpChange(changes, c.issueID), c.globalEvent)
	}
	return nil
}

// updateExpectations updates the current overall expectations with the changes
// provided. The expectations are the sum of all change records in the database.
// We continously keep track of that sum as new change records are added.
func (c *cloudExpStore) updateExpectations(tx *datastore.Transaction, changes map[string]types.TestClassification) error {
	currentExp, err := c.getCurrentExpectations(tx)
	if err != nil {
		return err
	}
	currentExp.update(changes)
	_, err = c.putCurrentExpectations(tx, currentExp)
	return err
}

// getCurrentExpectations retrieves the current expectations.
func (c *cloudExpStore) getCurrentExpectations(tx *datastore.Transaction) (ExpCollection, error) {
	_, ret, err := c.getTestDigestExps(tx, c.summaryKey)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// putCurrentExpectations stores the complete current expectations (contained in ExpCollection) in the datastore.
func (c *cloudExpStore) putCurrentExpectations(tx *datastore.Transaction, currentExp ExpCollection) ([]*datastore.PendingKey, error) {
	keys, err := currentExp.getKeys(c.testDigestExpEntity, c.summaryKey)
	if err != nil {
		return nil, err
	}
	return tx.PutMulti(keys, currentExp)
}

// getExpChangeKeys returns the keys of all expectation changes for the given issue
// in reverse chronological order.
func (c *cloudExpStore) getExpChangeKeys(atBeforeID int64) ([]*datastore.Key, error) {
	// Qquery all changes
	var egroup errgroup.Group
	var queryKeys []*datastore.Key
	egroup.Go(func() error {
		q := ds.NewQuery(c.changeEntity).
			Filter("OK =", true).
			KeysOnly()

		if c.issueID > 0 {
			q = q.Filter("IssueID =", c.issueID)
		}

		var err error
		queryKeys, err = c.client.GetAll(context.Background(), q, nil)
		return err
	})

	// Load the recent added changes.
	var recentKeys dsutil.KeySlice
	egroup.Go(func() error {
		var err error
		recentKeys, err = c.listHelper.GetRecent()
		return err
	})

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	ret := recentKeys.Merge(queryKeys)

	// Remove all keys that are newer than the target key
	if atBeforeID > 0 {
		// Filter out the target key if it exists.
		lastIdx := 0
		for _, key := range ret {
			if key.ID != atBeforeID {
				ret[lastIdx] = key
				lastIdx++
			}
		}
		ret = ret[:lastIdx]

		// Filter out all keys that are newer or the same time as the target key.
		ts := dsutil.GetTimeFromID(atBeforeID)
		for idx, key := range ret {
			if dsutil.GetTimeFromID(key.ID) <= ts {
				ret = ret[idx:]
				break
			}
		}
	}

	return ret, nil
}

// getTestDigstExpectations gets all expectations for the given change.
func (c *cloudExpStore) getTestDigestExps(tx *datastore.Transaction, parentKey *datastore.Key) ([]*datastore.Key, ExpCollection, error) {
	q := ds.NewQuery(c.testDigestExpEntity).Ancestor(parentKey)

	if tx != nil {
		q = q.Transaction(tx)
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
