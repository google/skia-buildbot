package expstorage

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"time"

	"go.skia.org/infra/go/jsonutils"

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

// cloudExpStore implements the ExpectationsStore interface with the
// Google Cloud Datastore as its backend.
// Since the difference between storing expectations for the master branch and
// the tryjobs of a Gerrit CL is only the inclusion of an issue id, it also
// simultaneously supports storing expectations for Gerrit issues using
// the same interface and based on the same storage client.
//
// To separate concerns, we store overall expectations (i.e. for the master branch)
// in the ds.MASTER_EXP_CHANGE and ds.MASTER_TEST_DIGEST_EXP entities.
// Expectations for Gerrit issues are stored in the ds.TRYJOB_EXP_CHANGE and
// ds.TRYJOB_TEST_DIGEST_EXP entities.
//
// We use instances of TDESlice to record both, expectations and expectation changes.
// These are usually stored as child entities.
//
// Expectation changes are stored as immutable, timestamped instances of
// 'ExpChange', which then act as parents to instances of TDESlice.
//
// Expectations are stored as children of summary entities. We maintain one
// summary entity for each logical expectation store, i.e. one for the master
// and one for each Gerrit issue.
// The summary entities also keep track of recently added expectation changes
// to provide a consistent listing.
//
// When expectation change, events of type EV_EXPSTORAGE_CHANGED and
// EV_TRYJOB_EXP_CHANGED are fired for the master branch and Gerrit issues
// respectively.
// Both events contain instances of EventExpectationChange as their payload.
//
type cloudExpStore struct {
	// issueID is the id of the Gerrit issue and <0 for other expectations,
	// i.e. the master branch
	issueID int64

	client   *datastore.Client
	eventBus eventbus.EventBus

	// recentKeysList keeps track of recently added changes. This allows to
	// provide a consistent listing of changes.
	recentKeysList *dsutil.RecentKeysList

	// summaryKey is the key of the summary entity which stores the keys of
	// recent changes and acts as the parent entity for overall expecations.
	summaryKey *datastore.Key

	// Use different entities depending on whether this manages expectations
	// for the master or a Gerrit issue
	changeEntity        ds.Kind
	testDigestExpEntity ds.Kind

	// eventExpChange keeps track of which event to fire when the expectations change.
	eventExpChange string

	// globalEvent keeps track whether we want to send a events within this instance
	// or on a global event bus.
	globalEvent bool
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

	// Create the instance for the master and set the target entities for the
	// master branch.
	summaryKey := ds.NewKey(ds.HELPER_RECENT_KEYS)
	summaryKey.Name = "expstorage-master"
	store := &cloudExpStore{
		issueID:             masterIssueID,
		changeEntity:        ds.MASTER_EXP_CHANGE,
		testDigestExpEntity: ds.MASTER_TEST_DIGEST_EXP,
		eventExpChange:      EV_EXPSTORAGE_CHANGED,
		globalEvent:         true,
		client:              client,
		eventBus:            eventBus,
		summaryKey:          summaryKey,
		recentKeysList:      dsutil.NewRecentKeysList(client, summaryKey, dsutil.DefaultConsistencyDelta),
	}

	// The factory allows to create an isolated ExpectationStore instance for the
	// given issue.
	factory := func(issueID int64) ExpectationsStore {
		summaryKey := ds.NewKey(ds.HELPER_RECENT_KEYS)
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
			recentKeysList:      dsutil.NewRecentKeysList(client, summaryKey, dsutil.DefaultConsistencyDelta),
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
func (c *cloudExpStore) AddChange(changes map[string]types.TestClassification, userID string) error {
	return c.makeChange(changes, userID, 0)
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
	var detailRecs [][]*TriageDetail
	var egroup errgroup.Group
	egroup.Go(func() error {
		expChanges := make([]*ExpChange, len(retKeys))
		if err := c.client.GetMulti(context.TODO(), retKeys, expChanges); err != nil {
			return sklog.FmtErrorf("Error retrieving expectation changes: %s", err)
		}

		for _, change := range expChanges {
			ret = append(ret, &TriageLogEntry{
				ID:           jsonutils.Number(change.ChangeID.ID),
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
					_, expColl, err := c.getTestDigestExps(nil, parentKey, false)
					if err != nil {
						return sklog.FmtErrorf("Error retrieving change details for %d: %s", parentKey.ID, err)
					}

					triageDetails := make([]*TriageDetail, 0, len(expColl[0].Names))
					for _, batch := range expColl {
						for nameIdx, name := range batch.Names {
							triageDetails = append(triageDetails, &TriageDetail{
								TestName: name,
								Digest:   batch.Digests[nameIdx],
								Label:    batch.Labels[nameIdx].String(),
							})
						}
					}

					sort.Slice(triageDetails, func(i, j int) bool {
						return (triageDetails[i].TestName < triageDetails[j].TestName) ||
							((triageDetails[i].TestName == triageDetails[j].TestName) &&
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

	// Fetch the change and its details.
	expChange := &ExpChange{}
	expChangeKey := ds.NewKey(c.changeEntity)
	expChangeKey.ID = changeID
	var egroup errgroup.Group
	egroup.Go(func() error {
		if err := c.client.Get(context.TODO(), expChangeKey, expChange); err != nil {
			if err == datastore.ErrNoSuchEntity {
				return sklog.FmtErrorf("Change with id %d does not exist.", changeID)
			}
			return sklog.FmtErrorf("Error retrieving change %d: %s", expChangeKey.ID, err)
		}
		return nil
	})

	var changes map[string]types.TestClassification = nil
	egroup.Go(func() error {
		var err error
		var expSlice TDESlice
		if _, expSlice, err = c.getTestDigestExps(nil, expChangeKey, false); err != nil {
			return sklog.FmtErrorf("Error retrieving expectation changes for change %d", changeID)
		}
		changes = expSlice.toExpectations(false).Tests
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

	// Retrieve the keys of all changes prior to the one we want to undo to
	// build the expectations at the time of the original change
	prevChangeKeys, err := c.getExpChangeKeys(changeID)
	if err != nil {
		return nil, sklog.FmtErrorf("Error retrieving keys for expectation changes: %s", err)
	}

	details := make([]TDESlice, len(prevChangeKeys))
	for idx, prevChangeKey := range prevChangeKeys {
		func(idx int, prevChangeKey *datastore.Key) {
			egroup.Go(func() error {
				var err error
				_, details[idx], err = c.getTestDigestExps(nil, prevChangeKey, false)
				return err
			})
		}(idx, prevChangeKey)
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
					changes[name][digest] = batch.Labels[idx]
				}
			}
		}
	}
	return changes, c.makeChange(changes, userID, changeID)
}

// TODO(stephana): The removeChange function is obsolete and should be removed.
// It was used for testing before the UndoChange function was added. It is simply
// wrong to change the expectations without a change record being added.

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
	if _, err := c.client.RunInTransaction(context.TODO(), updateFn); err != nil {
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
	ctx := context.TODO()
	timeStampMs := util.TimeStamp(time.Millisecond)
	expChange := &ExpChange{
		IssueID:      c.issueID,
		UserID:       userId,
		UndoChangeID: undoChangeID,
		TimeStamp:    timeStampMs,
		OK:           false,
	}

	// Add a new change record with the OK flag set to false. This
	// allows us to create change records outside of the transaction and
	// potentially in parallel without the write limits of doing it in a
	// transaction. The change record is not valid (= included in
	// searches until the OK flag is set to true inside the transaction below).
	changeKey := dsutil.TimeSortableKey(c.changeEntity, timeStampMs)
	if _, err = c.client.Put(ctx, changeKey, expChange); err != nil {
		return err
	}

	// If we have an error it means the transaction below failed and we want
	// to delete the part that was created outside of the transaction.
	defer func() {
		if err != nil {
			go c.deleteExpChange(changeKey)
		}
	}()

	// Write the changed expectations to the database.
	testChanges := buildTDESlice(changes)
	tcKeys := testChanges.getKeys(c.testDigestExpEntity, changeKey)
	if _, err = c.client.PutMulti(ctx, tcKeys, testChanges); err != nil {
		return err
	}

	updateFn := func(tx *datastore.Transaction) error {
		// Start transaction to:
		//  - store the key of the new change record to deal with eventual consistency
		//  - add the change to the summary
		//  - mark the change as valid.

		// Update the recent changes so we get full consistency on queries.
		if err := c.recentKeysList.Add(tx, changeKey); err != nil {
			return err
		}

		// Update the overall expectations.
		if err := c.updateExpectations(tx, changes); err != nil {
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
		return sklog.FmtErrorf("Error updating expecations and recentKeysList for change %d: %s", changeKey.ID, err)
	}

	if c.eventBus != nil {
		c.eventBus.Publish(c.eventExpChange, evExpChange(changes, c.issueID), c.globalEvent)
	}
	return nil
}

// updateExpectations updates the current overall expectations with the changes
// provided. The expectations are the sum of all change records in the database.
// We continuously keep track of that sum as new change records are added.
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
func (c *cloudExpStore) getCurrentExpectations(tx *datastore.Transaction) (TDESlice, error) {
	_, ret, err := c.getTestDigestExps(tx, c.summaryKey, false)
	if err != nil {
		return nil, sklog.FmtErrorf("Error retrieving expectations: %s", err)
	}
	return ret, nil
}

// putCurrentExpectations stores the complete current expectations (contained in the TDESlice instance) in the datastore.
func (c *cloudExpStore) putCurrentExpectations(tx *datastore.Transaction, currentExp TDESlice) ([]*datastore.PendingKey, error) {
	keys := currentExp.getKeys(c.testDigestExpEntity, c.summaryKey)
	return tx.PutMulti(keys, currentExp)
}

// getExpChangeKeys returns the keys of all expectation changes for the given issue
// in reverse chronological order. If beforeID is larger than 0 it is assumed to be
// an ID that was created via TimeSortableKey and we only want to retrieve keys that are
// older than the time stamp encoded in beforeID.
// The time is extracted with the GetTimeFromID function.
func (c *cloudExpStore) getExpChangeKeys(beforeID int64) ([]*datastore.Key, error) {
	// Query all changes
	var egroup errgroup.Group
	var queryKeys []*datastore.Key
	ctx := context.TODO()
	egroup.Go(func() error {
		q := ds.NewQuery(c.changeEntity).
			Filter("OK =", true).
			KeysOnly()

		if c.issueID > 0 {
			q = q.Filter("IssueID =", c.issueID)
		}

		var err error
		queryKeys, err = c.client.GetAll(ctx, q, nil)
		return err
	})

	// Load the recent added changes.
	var recently *dsutil.Recently
	egroup.Go(func() error {
		var err error
		// Get the recently changed keys. Note: these are added/removed in a
		// transaction so we are guaranteed they their OK value is true.
		recently, err = c.recentKeysList.GetRecent()
		return err
	})

	if err := egroup.Wait(); err != nil {
		return nil, sklog.FmtErrorf("Error retrieving keys of expectation changes: %s", err)
	}

	// Combine the recent keys with the result of the query for a consistent list
	// of the keys. ret will be sorted.
	ret := recently.Combine(queryKeys)

	// Remove all keys that are newer than the target key
	if beforeID > 0 {
		// Find keys that are strictly older than the given ID.
		beforeTS := dsutil.GetTimeFromID(beforeID)
		idx := sort.Search(len(ret), func(i int) bool {
			return dsutil.GetTimeFromID(ret[i].ID) < beforeTS
		})
		ret = ret[idx:]
	}

	return ret, nil
}

// getTestDigestExps gets all expectations for the given change. If
// keysOnly is true it will only return the keys of the entities that store
// the expectations.
func (c *cloudExpStore) getTestDigestExps(tx *datastore.Transaction, parentKey *datastore.Key, keysOnly bool) ([]*datastore.Key, TDESlice, error) {
	q := ds.NewQuery(c.testDigestExpEntity).Ancestor(parentKey)

	if tx != nil {
		q = q.Transaction(tx)
	}

	if keysOnly {
		q = q.KeysOnly()
	}

	var exps TDESlice
	expsKeys, err := c.client.GetAll(context.TODO(), q, &exps)
	if err != nil {
		return nil, nil, sklog.FmtErrorf("Error retrieving expectations: %s", err)
	}
	return expsKeys, exps, nil
}

// deleteExpChanges deletes the given expectation changes and logs errors.
// This is intended to clean up expectation changes that obsolete.
func (c *cloudExpStore) deleteExpChange(key *datastore.Key) {
	// Delete any expectations that have been added to this change.
	childrenKeys, _, err := c.getTestDigestExps(nil, key, true)
	if err != nil {
		sklog.Errorf("Error deleting expectation change %d: %s", key.ID, err)
		childrenKeys = nil
	}

	if err := c.client.DeleteMulti(context.TODO(), append(childrenKeys, key)); err != nil {
		sklog.Errorf("Error deleting expectation change %d: %s", key.ID, err)
	}
}
