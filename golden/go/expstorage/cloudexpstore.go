package expstorage

import (
	"context"
	"fmt"
	"sort"
	"strconv"
	"sync"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/eventbus"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/dsutil"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
)

const (
	// EV_TRYJOB_EXP_CHANGED is the event type that is fired when the expectations
	// for an issue change. It sends an instance of *TryjobExpChange.
	EV_TRYJOB_EXP_CHANGED = "expstorage:tryjob-exp-change"
)

// CloudExpStore implements the ExpectationsStore interface with the
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
type CloudExpStore struct {
	// issueID is the id of the Gerrit issue and <0 for other expectations,
	// i.e. the master branch
	issueID int64

	client   *datastore.Client
	eventBus eventbus.EventBus

	// recentKeysList keeps track of recently added changes. This allows to
	// provide a consistent listing of changes.
	recentKeysList *dsutil.RecentKeysList

	// summaryKey is the key of the summary entity which stores the keys of
	// recent changes and acts as the parent entity for overall expectations.
	summaryKey *datastore.Key

	expectationsKey *datastore.Key
	blobStore       *dsutil.BlobStore

	// Use different entities depending on whether this manages expectations
	// for the master or a Gerrit issue
	changeKind ds.Kind

	// eventExpChange keeps track of which event to fire when the expectations change.
	eventExpChange string

	// globalEvent keeps track whether we want to send a events within this instance
	// or on a global event bus.
	globalEvent bool

	// lastTS and tsMutex ensure that we get distinct timestamps with ms granularity
	lastTS  int64
	tsMutex sync.Mutex
}

// IssueExpStoreFactory creates an DEPRECATED_ExpectationsStore instance for the given issue id.
type IssueExpStoreFactory func(issueID int64) DEPRECATED_ExpectationsStore

// NewCloudExpectationsStore returns an ExpectationsStore implementation based on
// Cloud Datastore for the master branch and a factory to create ExpectationsStore
// instances for Gerrit issues. The factory uses the same datastore client as the
// master store.
func NewCloudExpectationsStore(client *datastore.Client, eventBus eventbus.EventBus) (*CloudExpStore, IssueExpStoreFactory, error) {
	if client == nil {
		return nil, nil, sklog.FmtErrorf("Received nil for datastore client.")
	}

	// Create the instance for the master and set the target entities for the
	// master branch.
	summaryKey := ds.NewKey(ds.HELPER_RECENT_KEYS)
	summaryKey.Name = "expstorage-recent-keys-master"
	expectationsKey := ds.NewKey(ds.EXPECTATIONS_BLOB_ROOT)
	expectationsKey.Name = "expstorage-expectations-master"
	blobStore := dsutil.NewBlobStore(client, ds.EXPECTATIONS_BLOB_ROOT, ds.EXPECTATIONS_BLOB)

	store := &CloudExpStore{
		issueID:         masterIssueID,
		changeKind:      ds.MASTER_EXP_CHANGE,
		eventExpChange:  EV_EXPSTORAGE_CHANGED,
		globalEvent:     true,
		client:          client,
		eventBus:        eventBus,
		summaryKey:      summaryKey,
		expectationsKey: expectationsKey,
		recentKeysList:  dsutil.NewRecentKeysList(client, summaryKey, dsutil.DefaultConsistencyDelta),
		blobStore:       blobStore,
	}

	// The factory allows to create an isolated ExpectationStore instance for the
	// given issue.
	factory := func(issueID int64) DEPRECATED_ExpectationsStore {
		summaryKey := ds.NewKey(ds.HELPER_RECENT_KEYS)
		summaryKey.Name = fmt.Sprintf("expstorage-issue-%d", issueID)
		expectationsKey := ds.NewKey(ds.EXPECTATIONS_BLOB_ROOT)
		expectationsKey.Name = fmt.Sprintf("expstorage-expectations-issue-%d", issueID)
		return &CloudExpStore{
			issueID:         issueID,
			changeKind:      ds.TRYJOB_EXP_CHANGE,
			eventExpChange:  EV_TRYJOB_EXP_CHANGED,
			globalEvent:     false,
			client:          client,
			eventBus:        eventBus,
			summaryKey:      summaryKey,
			expectationsKey: expectationsKey,
			recentKeysList:  dsutil.NewRecentKeysList(client, summaryKey, dsutil.DefaultConsistencyDelta),
			blobStore:       blobStore,
		}
	}

	// Check the connection to the cloud datastore and if we could load the
	// expectations successfully.
	_, _, err := store.loadCurrentExpectations(nil)
	if err != nil {
		return nil, nil, sklog.FmtErrorf("Error in test call to the cloud datastore: %s", err)
	}
	return store, factory, nil
}

// Get implements the ExpectationsStore interface.
func (c *CloudExpStore) Get() (types.Expectations, error) {
	expectations, _, err := c.loadCurrentExpectations(nil)
	if err != nil {
		return nil, sklog.FmtErrorf("Error retrieving expectations: %s", err)
	}
	return expectations, nil
}

// AddChange implements the ExpectationsStore interface.
func (c *CloudExpStore) AddChange(changes types.Expectations, userID string) error {
	_, err := c.makeChange(changes, userID, c.getUniqueTimeStampMs(), 0, true)
	return err
}

// ImportChange bypasses the ExpectationStore interface to copy change records directly.
func (c *CloudExpStore) ImportChange(changes types.Expectations, userID string, timeStamp int64) (*datastore.Key, error) {
	return c.makeChange(changes, userID, timeStamp, 0, false)
}

// QueryLog implements the ExpectationsStore interface.
func (c *CloudExpStore) QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error) {
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
	expChanges := make([]*ExpChange, len(retKeys))
	if err := c.client.GetMulti(context.TODO(), retKeys, expChanges); err != nil {
		return nil, 0, sklog.FmtErrorf("Error retrieving expectation changes: %s", err)
	}

	for _, change := range expChanges {
		ret = append(ret, &TriageLogEntry{
			ID:           strconv.FormatInt(change.ChangeID.ID, 10),
			Name:         change.UserID,
			TS:           change.TimeStamp,
			ChangeCount:  int(change.Count),
			Details:      nil,
			UndoChangeID: change.UndoChangeID,
		})
	}

	// If we want details fetch them in parallel.
	var egroup errgroup.Group
	var detailRecs [][]*TriageDetail
	if details {
		detailRecs = make([][]*TriageDetail, len(retKeys))
		for idx, expChange := range expChanges {
			func(idx int, blobKey *datastore.Key) {
				egroup.Go(func() error {
					exp := types.Expectations{}
					if err := c.blobStore.Load(blobKey, &exp); err != nil {
						return err
					}

					triageDetails := make([]*TriageDetail, 0, len(exp))
					for testName, digests := range exp {
						for digest, label := range digests {
							triageDetails = append(triageDetails, &TriageDetail{
								TestName: testName,
								Digest:   digest,
								Label:    label.String(),
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
			}(idx, expChange.ExpectationsBlob)
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
func (c *CloudExpStore) UndoChange(changeID int64, userID string) (types.Expectations, error) {
	// Make sure the entity is valid.
	if changeID <= 0 {
		return nil, sklog.FmtErrorf("Change with id %d does not exist.", changeID)
	}

	// Fetch the change record of the change we want to undo.
	expChange := &ExpChange{}
	expChangeKey := ds.NewKey(c.changeKind)
	expChangeKey.ID = changeID
	if err := c.client.Get(context.TODO(), expChangeKey, expChange); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, sklog.FmtErrorf("Change with id %d does not exist.", changeID)
		}
		return nil, sklog.FmtErrorf("Error retrieving change %d: %s", expChangeKey.ID, err)
	}

	// Fetch the actual changes.
	undoChanges := types.Expectations{}
	if err := c.blobStore.Load(expChange.ExpectationsBlob, &undoChanges); err != nil {
		return nil, sklog.FmtErrorf("Error retrieving expectations blob: %s", err)
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

	// Build the expectations at that point.
	prevExp, err := c.CalcExpectations(prevChangeKeys)
	if err != nil {
		return nil, sklog.FmtErrorf("Unable to get expectations for undo: %s", err)
	}

	changes := types.Expectations{}
	for testName, digests := range undoChanges {
		changes[testName] = make(types.TestClassification, len(digests))
		for digest := range digests {
			changes[testName][digest] = prevExp[testName][digest]
		}
	}

	_, err = c.makeChange(changes, userID, c.getUniqueTimeStampMs(), changeID, true)
	return changes, err
}

// Clear implements the ExpectationsStore interface.
func (c *CloudExpStore) Clear() error {
	ctx := context.TODO()
	delKeys := []*datastore.Key{c.summaryKey, c.expectationsKey}

	allExpChangeKeys, err := c.getExpChangeKeys(0)
	if err != nil {
		return sklog.FmtErrorf("Error retrieving keys for expectation changes: %s", err)
	}
	delBlobKeys := make([]*datastore.Key, 0, len(allExpChangeKeys)+1)

	_, state, err := c.loadCurrentExpectations(nil)
	if err != nil {
		return sklog.FmtErrorf("Error loading current expectations: %s", err)
	}
	delBlobKeys = append(delBlobKeys, state.ExpectationsBlob)

	// Extract the keys of the blobs storing the expectations.
	for _, key := range allExpChangeKeys {
		expChange := &ExpChange{}
		if err := c.client.Get(ctx, key, expChange); err != nil {
			return sklog.FmtErrorf("Error retrieving expectations change %d: %s", key.ID, err)
		}
		delBlobKeys = append(delBlobKeys, expChange.ExpectationsBlob)
	}

	// Add the expectation change keys  to the keys that need to be deleted.
	delKeys = append(delKeys, allExpChangeKeys...)

	var egroup errgroup.Group

	// Delete the expectations blobs.
	for _, key := range delBlobKeys {
		if key != nil {
			func(key *datastore.Key) {
				egroup.Go(func() error { return c.blobStore.Delete(key) })
			}(key)
		}
	}

	// Delete all keys we have accumulated. 500 at a time which the limit for
	// cloud datastore.
	for _, batch := range dsutil.Batch(delKeys, 500) {
		func(batch []*datastore.Key) {
			egroup.Go(func() error { return c.client.DeleteMulti(ctx, batch) })
		}(batch)
	}

	// Wait until it's all done.
	return egroup.Wait()
}

// PutExpectations writes the expectations directly to the datastore
func (c *CloudExpStore) PutExpectations(exps types.Expectations) error {
	return c.updateCurrentExpectations(nil, exps, true, nil)
}

// CalcExpectations calculates the expectations by accumulating the expectation changes
// referenced by the given list of keys. keys are assumed to be sorted in
// reverse chronological order.
func (c *CloudExpStore) CalcExpectations(keys []*datastore.Key) (types.Expectations, error) {
	concurrent := make(chan bool, 10000)
	changes := make([]types.Expectations, len(keys))
	var egroup errgroup.Group

	for idx, key := range keys {
		concurrent <- true
		func(idx int, key *datastore.Key) {
			egroup.Go(func() error {
				defer func() {
					<-concurrent
				}()

				exps, err := c.getChanges(key)
				changes[idx] = exps
				return err
			})
		}(idx, key)
	}
	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	ret := types.Expectations{}
	for i := len(changes) - 1; i >= 0; i-- {
		ret.MergeExpectations(changes[i])
	}
	return ret, nil
}

// RemoveChange implements the DebugExpectationsStore interface.
func (c *CloudExpStore) RemoveChange(changes types.Expectations) (err error) {
	for _, digests := range changes {
		for digest := range digests {
			digests[digest] = types.UNTRIAGED
		}
	}

	// Set it up so that transaction related functions are executed after the
	// transaction finishes.
	actions := dsutil.TxActions{}
	defer func() { actions.Run(err) }()
	updateFn := func(tx *datastore.Transaction) error {
		return c.updateCurrentExpectations(tx, changes, false, &actions)
	}

	// Run the removal changes in a transaction.
	if _, err := c.client.RunInTransaction(context.TODO(), updateFn); err != nil {
		return err
	}

	if c.eventBus != nil {
		// This is always a local event since it's only used for testing.
		c.eventBus.Publish(c.eventExpChange, evExpChange(changes, c.issueID, nil), false)
	}
	return nil
}

// makeChange updates the expectations by adding a new change record to the datastore.
// timeStampMs is the timestamp of this change.
// If undoChangeId is larger than 0 then it will be recorded in the change record
// since this is an undo of an earlier change.
// If transactional is true it the change will be added in a transaction.
// This should only be false when we import existing data.
func (c *CloudExpStore) makeChange(changes types.Expectations, userId string, timeStampMs int64, undoChangeID int64, transactional bool) (changeKey *datastore.Key, err error) {
	ctx := context.TODO()

	// Get the total count of changes so we can include it in the change record.
	count := 0
	for _, digests := range changes {
		count += len(digests)
	}

	// Write the expectation changes.
	blobKey, err := c.blobStore.Save(changes)
	if err != nil {
		return nil, sklog.FmtErrorf("Saving changes to blob failed: %s", err)
	}

	// If we have an error it means the transaction below failed and we want
	// to delete the part that was created outside of the transaction.
	purgeKeys := []*datastore.Key(nil)
	actions := dsutil.TxActions{}
	actions.AddRollbackFn(func() error { return c.blobStore.Delete(blobKey) })
	actions.AddRollbackFn(func() error { return c.client.DeleteMulti(ctx, purgeKeys) })
	defer func() { actions.Run(err) }()

	// Add a new change record with the OK flag set to false. This
	// allows us to create change records outside of the transaction and
	// potentially in parallel without the write limits of doing it in a
	// transaction. The change record is not valid (= included in
	// searches until the OK flag is set to true inside the transaction below).
	changeKey = dsutil.TimeSortableKey(c.changeKind, timeStampMs)
	expChange := &ExpChange{
		IssueID:          c.issueID,
		UserID:           userId,
		UndoChangeID:     undoChangeID,
		TimeStamp:        timeStampMs,
		OK:               false,
		ExpectationsBlob: blobKey,
		Count:            int64(count),
	}
	if changeKey, err = c.client.Put(ctx, changeKey, expChange); err != nil {
		return nil, sklog.FmtErrorf("Error writing change record: %s", err)
	}
	purgeKeys = append(purgeKeys, changeKey)

	updateFn := func(tx *datastore.Transaction) error {
		// Start transaction to:
		//  - store the key of the new change record to deal with eventual consistency
		//  - add the change to the summary
		//  - mark the change as valid.

		// Update the recent changes so we get full consistency on queries.
		if err := c.recentKeysList.Add(tx, changeKey); err != nil {
			return err
		}

		// Update the overall expectations
		if err := c.updateCurrentExpectations(tx, changes, false, &actions); err != nil {
			return err
		}

		// Mark the expectation change as valid.
		expChange.OK = true
		_, err := tx.Put(changeKey, expChange)
		return err
	}

	// Run the relevant updates in a transaction.
	if transactional {
		if _, err = c.client.RunInTransaction(ctx, updateFn); err != nil {
			return nil, sklog.FmtErrorf("Error updating expectations and recentKeysList for change %d: %s", changeKey.ID, err)
		}
	} else {
		expChange.OK = true
		if _, err = c.client.Mutate(ctx, datastore.NewUpdate(changeKey, expChange)); err != nil {
			return nil, sklog.FmtErrorf("Error commiting the expectation change: %s", err)
		}
	}

	if c.eventBus != nil {
		c.eventBus.Publish(c.eventExpChange, evExpChange(changes, c.issueID, nil), c.globalEvent)
	}
	return changeKey, nil
}

// updateCurrentExpectations updates the current overall expectations with the changes
// provided. The expectations are the sum of all change records in the database.
// We continuously keep track of that sum as new change records are added.
func (c *CloudExpStore) updateCurrentExpectations(tx *datastore.Transaction, changes types.Expectations, overwrite bool, actions *dsutil.TxActions) (err error) {
	currentExp, expState, err := c.loadCurrentExpectations(tx)
	if err != nil {
		return sklog.FmtErrorf("Error loading current expectations: %s", err)
	}
	oldExpsBlob := expState.ExpectationsBlob

	if overwrite || (currentExp == nil) {
		currentExp = changes.DeepCopy()
	} else {
		currentExp.MergeExpectations(changes)
	}

	// Create a new entry for the expectations
	newBlobKey, err := c.blobStore.Save(currentExp)
	if err != nil {
		return sklog.FmtErrorf("Error writing new expectations: %s", err)
	}

	// delete the new blob if we fail
	delNewBlobFn := func() error {
		if err := c.blobStore.Delete(newBlobKey); err != nil {
			return sklog.FmtErrorf("Error deleting new expectations blob: %s", err)
		}
		return nil
	}

	// either at the very end of this function or as part of the transaction
	if tx == nil {
		defer func() {
			if err != nil {
				util.LogErr(delNewBlobFn())
			}
		}()
	} else {
		actions.AddRollbackFn(delNewBlobFn)
	}

	// Write the new key to our expectation state
	expState.ExpectationsBlob = newBlobKey

	putFn := dsutil.PutFn(c.client, tx)
	if err = putFn(c.expectationsKey, expState); err != nil {
		return sklog.FmtErrorf("Error writing new expectations blob: %s", err)
	}

	// If there is not old blob to be deleted we are done
	if oldExpsBlob == nil {
		return nil
	}

	// Remove the old blob either right away or after the transaction succeeds
	delOldBlob := func() error {
		if err := c.blobStore.Delete(oldExpsBlob); err != nil {
			return sklog.FmtErrorf("Error removing old expectations blob: %s", err)
		}
		return nil
	}
	if tx == nil {
		if err := delOldBlob(); err != nil {
			sklog.Errorf("Error deleting old blob data: %s", err)
		}
		return nil
	}
	actions.AddCommitFn(delOldBlob)
	return nil
}

// getExpChangeKeys returns the keys of all expectation changes for the given issue
// in reverse chronological order. If beforeID is larger than 0 it is assumed to be
// an ID that was created via TimeSortableKey and we only want to retrieve keys that are
// older than the time stamp encoded in beforeID.
// The time is extracted with the GetTimeFromID function.
func (c *CloudExpStore) getExpChangeKeys(beforeID int64) ([]*datastore.Key, error) {
	// Query all changes
	var egroup errgroup.Group
	var queryKeys []*datastore.Key
	ctx := context.TODO()
	egroup.Go(func() error {
		q := ds.NewQuery(c.changeKind).
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

// loadCurrentExpectations loads the current expectations for this expectation
// store (either for the master branch or for a Gerrit issue). If no expectations
// have been set it will return non-nil values and no error.
func (c *CloudExpStore) loadCurrentExpectations(tx *datastore.Transaction) (types.Expectations, *expectationsState, error) {
	getFn := dsutil.GetFn(c.client, tx)
	exp := types.Expectations{}
	expState := &expectationsState{}
	if err := getFn(c.expectationsKey, expState); err != nil && err != datastore.ErrNoSuchEntity {
		return nil, nil, err
	}

	var err error
	if expState.ExpectationsBlob != nil {
		if err = c.blobStore.Load(expState.ExpectationsBlob, &exp); err != nil {
			return nil, nil, err
		}
	}

	return exp, expState, err
}

// getChanges loads the changes for the given expectations change key.
func (c *CloudExpStore) getChanges(expChangeKey *datastore.Key) (types.Expectations, error) {
	ctx := context.TODO()
	expChange := &ExpChange{}
	if err := c.client.Get(ctx, expChangeKey, expChange); err != nil {
		return nil, err
	}

	ret := types.Expectations{}
	if expChange.ExpectationsBlob != nil {
		if err := c.blobStore.Load(expChange.ExpectationsBlob, &ret); err != nil {
			return nil, sklog.FmtErrorf("Unable to load expectations blob: %s", err)
		}
	}
	return ret, nil
}

// getUniqueTimeStampMs returns a unique time in milliseconds
func (c *CloudExpStore) getUniqueTimeStampMs() int64 {
	c.tsMutex.Lock()
	defer c.tsMutex.Unlock()
	ts := util.TimeStampMs()
	if ts <= c.lastTS {
		ts = c.lastTS + 1
	}
	c.lastTS = ts
	return ts
}

// Make sure CloudExpStore fulfills the ExpectationsStore interface
var _ ExpectationsStore = (*CloudExpStore)(nil)

// Make sure CloudExpStore fulfills the DEPRECATED_ExpectationsStore interface
var _ DEPRECATED_ExpectationsStore = (*CloudExpStore)(nil)
