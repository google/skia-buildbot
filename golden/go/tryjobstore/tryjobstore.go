package tryjobstore

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync/atomic"
	"time"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/eventbus"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
)

const (
	// EV_TRYJOB_EXP_CHANGED is the event type that is fired when the expectations
	// for an issue change. It sends an instance of *TryjobExpChange.
	EV_TRYJOB_EXP_CHANGED = "tryjobstore:change"
)

// TryjobStore define methods to store tryjob information and code review
// issues as a key component for for transactional trybot support.
type TryjobStore interface {
	// ListIssues lists all current issues in the store.
	ListIssues() ([]*Issue, int, error)

	// GetIssue retrieves information about the given issue and patchsets. If needded
	// this will include tryjob information.
	GetIssue(issueID int64, loadTryjobs bool, targetPatchsets []int64) (*IssueDetails, error)

	// UpdateIssue updates the given issue with the provided data. If the issue does not
	// exist in the database it will be created.
	UpdateIssue(details *IssueDetails) error

	// DeleteIssue deletes the given issue and related information.
	DeleteIssue(issueID int64) error

	// GetTryjob returns the Tryjob instance defined by issueID and buildBucketID.
	GetTryjob(issueID, buildBucketID int64) (*Tryjob, error)

	// GetTryjobResults returns the results for the given issue and patchsets.
	// If filterDup is true only the newest results (by patchset) are returned,
	// e.g. if the 'Test-Some-Platform' builder is run for patchset 1 and for
	// patchset 5, only the results of the latter run are included.
	GetTryjobResults(issueID int64, patchsetID []int64, filterDup bool) ([]*Tryjob, [][]*TryjobResult, error)

	// UpdateTryjob updates the information about a tryjob. If the tryjob does not
	// exist it will be created.
	UpdateTryjob(issueID int64, tryjob *Tryjob) error

	// UpdateTryjobResult updates the results for the given tryjob.
	UpdateTryjobResult(tryjob *Tryjob, results []*TryjobResult) error

	// AddChange adds an update to the expectations provided by a user.
	AddChange(issueID int64, changes map[string]types.TestClassification, userID string) error

	// GetExpectations gets the expectations for the tryjob result of the given issue.
	GetExpectations(issueID int64) (exp *expstorage.Expectations, err error)

	// UndoChange reverts a previous expectations change for this issue.
	UndoChange(issueID int64, changeID int64, userID string) (map[string]types.TestClassification, error)

	// QueryLog returns a list of expectation changes for the given issue.
	QueryLog(issueID int64, offset, size int, details bool) ([]*expstorage.TriageLogEntry, int, error)
}

const (
	// batchsize is the maximal size of entities processed in a single batch. This
	// is to stay below the limit of 500 and empirically keeps transactions small
	// enough to stay below 10MB limit.
	batchSize = 300
)

// cloudTryjobStore implements the TryjobStore interface on top of cloud datastore.
type cloudTryjobStore struct {
	client   *datastore.Client
	eventBus eventbus.EventBus
}

// NewCloudTryjobStore creates a new instance of TryjobStore based on cloud datastore.
func NewCloudTryjobStore(client *datastore.Client, eventBus eventbus.EventBus) (TryjobStore, error) {
	return &cloudTryjobStore{
		client:   client,
		eventBus: eventBus,
	}, nil
}

// ListIssues implements the TryjobStore interface.
func (c *cloudTryjobStore) ListIssues() ([]*Issue, int, error) {
	query := ds.NewQuery(ds.ISSUE).KeysOnly()
	keys, err := c.client.GetAll(context.Background(), query, nil)
	if err != nil {
		return nil, 0, err
	}

	ret := make([]*Issue, 0, len(keys))
	for _, k := range keys {
		ret = append(ret, &Issue{ID: k.ID})
	}
	return ret, len(ret), nil
}

// GetIssue implements the TryjobStore interface.
func (c *cloudTryjobStore) GetIssue(issueID int64, loadTryjobs bool, targetPatchsets []int64) (*IssueDetails, error) {
	target := &IssueDetails{}
	key := c.getIssueKey(issueID)
	ok, err := c.getEntity(key, target, nil)
	if err != nil {
		return nil, sklog.FmtErrorf("Error in getEntity for %s: %s", key, err)
	}

	if !ok {
		return nil, nil
	}

	if loadTryjobs {
		_, tryjobs, err := c.getTryjobsForIssue(issueID, nil, false)
		if err != nil {
			return nil, sklog.FmtErrorf("Error getting tryjobs for issue: %s", err)
		}

		for _, tj := range tryjobs {
			ps := target.findPatchset(tj.PatchsetID)
			if ps == nil {
				return nil, sklog.FmtErrorf("Unable to find patchset %d in issue %d:", tj.PatchsetID, target.ID)
			}
			ps.Tryjobs = append(ps.Tryjobs, tj)
		}

		for _, ps := range target.PatchsetDetails {
			func(tryjobs []*Tryjob) {
				sort.Slice(tryjobs, func(i, j int) bool {
					return tryjobs[i].BuildBucketID < tryjobs[j].BuildBucketID
				})
			}(ps.Tryjobs)
		}
	}

	return target, nil
}

// UpdateIssue implements the TryjobStore interface.
func (c *cloudTryjobStore) UpdateIssue(details *IssueDetails) error {
	return c.updateIfNewer(c.getIssueKey(details.ID), details)
}

// DeleteIssue implements the TryjobStore interface.
func (c *cloudTryjobStore) DeleteIssue(issueID int64) error {
	ctx := context.Background()
	key := c.getIssueKey(issueID)

	var egroup errgroup.Group

	egroup.Go(func() error {
		// Delete any tryjobs that are still there.
		return c.deleteTryjobsForIssue(issueID)
	})

	egroup.Go(func() error {
		// Delete all expectations for this issue.
		return c.deleteExpectationsForIssue(issueID)
	})

	// Make sure all dependents are deleted.
	if err := egroup.Wait(); err != nil {
		return err
	}

	// Delete the entity.
	return c.client.Delete(ctx, key)
}

// GetTryjob implements the TryjobStore interface.
func (c *cloudTryjobStore) GetTryjob(issueID, buildBucketID int64) (*Tryjob, error) {
	ret := &Tryjob{}
	if err := c.client.Get(context.Background(), c.getTryjobKey(buildBucketID), ret); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, nil
		}
		return nil, err
	}

	return ret, nil
}

// UpdateTryjob implements the TryjobStore interface.
func (c *cloudTryjobStore) UpdateTryjob(issueID int64, tryjob *Tryjob) error {
	return c.updateIfNewer(c.getTryjobKey(tryjob.BuildBucketID), tryjob)
}

// GetTryjobResults implements the TryjobStore interface.
func (c *cloudTryjobStore) GetTryjobResults(issueID int64, patchsetIDs []int64, filterDup bool) ([]*Tryjob, [][]*TryjobResult, error) {
	tryjobKeys, tryjobs, err := c.getTryjobsForIssue(issueID, patchsetIDs, false)
	if err != nil {
		return nil, nil, err
	}

	// Filter tryjob runs that have runs against newer patchsets.
	if filterDup {
		// sort tryjobs by patchset and when they were last updated.
		sort.Slice(tryjobKeys, func(i, j int) bool {
			return (tryjobs[i].PatchsetID < tryjobs[j].PatchsetID) ||
				((tryjobs[i].PatchsetID == tryjobs[j].PatchsetID) && (tryjobs[i].Updated.Before(tryjobs[j].Updated)))
		})

		// Iterate the builders in reverse order filter out duplicates by builder.
		builders := util.StringSet{}
		for i := len(tryjobs) - 1; i >= 0; i-- {
			if !builders[tryjobs[i].Builder] {
				builders[tryjobs[i].Builder] = true
				tryjobs[i] = nil
			}
		}

		newTryjobKeys := make([]*datastore.Key, 0, len(builders))
		newTryjobs := make([]*Tryjob, 0, len(builders))
		for idx, tryjob := range tryjobs {
			if tryjob != nil {
				newTryjobs = append(newTryjobs, tryjob)
				newTryjobKeys = append(newTryjobKeys, tryjobKeys[idx])
			}
		}
		tryjobs = newTryjobs
		tryjobKeys = newTryjobKeys
	}
	_, tryjobResults, err := c.getResultsForTryjobs(tryjobKeys, false)
	if err != nil {
		return nil, nil, err
	}

	return tryjobs, tryjobResults, nil
}

// UpdateTryjobResults implements the TryjobStore interface.
func (c *cloudTryjobStore) UpdateTryjobResult(tryjob *Tryjob, results []*TryjobResult) error {
	tryjobKey := c.getTryjobKey(tryjob.BuildBucketID)
	keys := make([]*datastore.Key, 0, len(results))
	uniqueEntries := util.StringSet{}
	for _, result := range results {
		// Make sure that tests are not bunched together.
		if len(result.Params[types.PRIMARY_KEY_FIELD]) != 1 {
			return fmt.Errorf("Parameter value for primary key field '%s' must exactly contain one value. Found: %v", types.PRIMARY_KEY_FIELD, result.Params[types.PRIMARY_KEY_FIELD])
		}

		keys = append(keys, c.getTryjobResultKey(tryjobKey))
		uniqueEntries[result.TestName+result.Digest] = true
	}

	if len(uniqueEntries) != len(keys) {
		return fmt.Errorf("All (test,digest) pairs must be unique when adding tryjob results.")
	}

	// var egroup errgroup.Group
	for i := 0; i < len(keys); i += batchSize {
		endIdx := util.MinInt(i+batchSize, len(keys))
		if _, err := c.client.PutMulti(context.Background(), keys[i:endIdx], results[i:endIdx]); err != nil {
			return err
		}
	}
	return nil
}

// AddChange implements the TryjobStore interface.
func (c *cloudTryjobStore) AddChange(issueID int64, changes map[string]types.TestClassification, userID string) (err error) {
	// Write the change record.
	ctx := context.Background()
	expChange := &ExpChange{
		IssueID:   issueID,
		UserID:    userID,
		TimeStamp: util.TimeStamp(time.Millisecond),
	}

	var changeKey *datastore.Key
	if changeKey, err = c.client.Put(ctx, ds.NewKey(ds.TRYJOB_EXP_CHANGE), expChange); err != nil {
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

	// Insert all the expectation changes.
	testChanges := make([]*TestDigestExp, 0, len(changes))
	for testName, classification := range changes {
		for digest, label := range classification {
			testChanges = append(testChanges, &TestDigestExp{
				Name:   testName,
				Digest: digest,
				Label:  label.String(),
			})
		}
	}

	tdeKeys := make([]*datastore.Key, len(testChanges), len(testChanges))
	for idx := range testChanges {
		key := ds.NewKey(ds.TEST_DIGEST_EXP)
		key.Parent = changeKey
		tdeKeys[idx] = key
	}

	if _, err = c.client.PutMulti(ctx, tdeKeys, testChanges); err != nil {
		return err
	}

	// Mark the expectation change as valid.
	expChange.OK = true
	if _, err = c.client.Put(ctx, changeKey, expChange); err != nil {
		return err
	}

	if c.eventBus != nil {
		c.eventBus.Publish(EV_TRYJOB_EXP_CHANGED, &IssueExpChange{IssueID: issueID}, false)
	}

	return nil
}

// GetExpectations implements the TryjobStore interface.
func (c *cloudTryjobStore) GetExpectations(issueID int64) (exp *expstorage.Expectations, err error) {
	// Get all expectation changes and iterate over them updating the result.
	expChangeKeys, _, err := c.getExpChangesForIssue(issueID, -1, -1, true)
	if err != nil {
		return nil, err
	}

	testChanges := make([][]*TestDigestExp, len(expChangeKeys), len(expChangeKeys))
	// Iterate over the expectations build the expectations.
	var egroup errgroup.Group
	for idx, key := range expChangeKeys {
		func(idx int, key *datastore.Key) {
			egroup.Go(func() error {
				_, testChanges[idx], err = c.getTestDigestExps(key, false)
				return err
			})
		}(idx, key)
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}

	ret := expstorage.NewExpectations()
	for _, expByChange := range testChanges {
		if len(expByChange) > 0 {
			for _, oneChange := range expByChange {
				ret.SetTestExpectation(oneChange.Name, oneChange.Digest, types.LabelFromString(oneChange.Label))
			}
		}
	}

	return ret, nil
}

// TODO(stephana): Implement the UndoChange and QueryLog methods once the corresponding
// endpoints in skiacorrectness exist.

// UndoChange implements the TryjobStore interface.
func (c *cloudTryjobStore) UndoChange(issueID int64, changeID int64, userID string) (map[string]types.TestClassification, error) {
	return nil, nil
}

// QueryLog implements the TryjobStore interface.
func (c *cloudTryjobStore) QueryLog(issueID int64, offset, size int, details bool) ([]*expstorage.TriageLogEntry, int, error) {
	// TODO(stephana): Optimize this so we don't make the first request just to obtain the total.
	allKeys, _, err := c.getExpChangesForIssue(issueID, -1, -1, true)
	if err != nil {
		return nil, 0, sklog.FmtErrorf("Error retrieving keys for expectation changes: %s", err)
	}

	_, expChanges, err := c.getExpChangesForIssue(issueID, offset, size, false)
	if err != nil {
		return nil, 0, sklog.FmtErrorf("Error retrieving expectation changes: %s", err)
	}

	ret := make([]*expstorage.TriageLogEntry, 0, len(expChanges))
	for _, change := range expChanges {
		ret = append(ret, &expstorage.TriageLogEntry{
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

// deleteTryjobsForIssue deletes all tryjob information for the given issue.
func (c *cloudTryjobStore) deleteTryjobsForIssue(issueID int64) error {
	// Get all the tryjob keys.
	tryjobKeys, _, err := c.getTryjobsForIssue(issueID, nil, true)
	if err != nil {
		return fmt.Errorf("Error retrieving tryjob keys: %s", err)
	}

	ctx := context.Background()
	// Delete all results of the tryjobs.
	tryjobResultKeys, _, err := c.getResultsForTryjobs(tryjobKeys, true)
	if err != nil {
		return err
	}

	for _, keys := range tryjobResultKeys {
		// Break the keys down in batches.
		for i := 0; i < len(keys); i += batchSize {
			currBatch := keys[i:util.MinInt(i+batchSize, len(keys))]
			if err := c.client.DeleteMulti(ctx, currBatch); err != nil {
				return fmt.Errorf("Error deleting tryjob results: %s", err)
			}
		}
	}

	// Delete the tryjobs themselves.
	if err := c.client.DeleteMulti(ctx, tryjobKeys); err != nil {
		return fmt.Errorf("Error deleting %d tryjobs for issue %d: %s", len(tryjobKeys), issueID, err)
	}
	return nil
}

// getResultsForTryjobs returns the test results for the given tryjobs.
func (c *cloudTryjobStore) getResultsForTryjobs(tryjobKeys []*datastore.Key, keysOnly bool) ([][]*datastore.Key, [][]*TryjobResult, error) {
	// Collect all results across tryjobs.
	n := len(tryjobKeys)
	tryjobResultKeys := make([][]*datastore.Key, n, n)
	var tryjobResults [][]*TryjobResult = nil
	if !keysOnly {
		tryjobResults = make([][]*TryjobResult, n, n)
	}

	// Get there keys and results.
	ctx := context.Background()
	var egroup errgroup.Group
	for idx, key := range tryjobKeys {
		func(idx int, key *datastore.Key) {
			egroup.Go(func() error {
				query := ds.NewQuery(ds.TRYJOB_RESULT).Ancestor(key)
				if keysOnly {
					query = query.KeysOnly()
				}

				queryResult := []*TryjobResult{}
				var err error
				if tryjobResultKeys[idx], err = c.client.GetAll(ctx, query, &queryResult); err != nil {
					return err
				}

				if !keysOnly {
					tryjobResults[idx] = queryResult
				}
				return nil
			})
		}(idx, key)
	}

	if err := egroup.Wait(); err != nil {
		return nil, nil, fmt.Errorf("Error getting tryjob results: %s", err)
	}

	return tryjobResultKeys, tryjobResults, nil
}

// deleteExpChanges deletes the given expectation changes.
func (c *cloudTryjobStore) deleteExpChanges(keys []*datastore.Key) error {
	return c.client.DeleteMulti(context.Background(), keys)
}

// deleteExpectationsForIssue deletes all expectations for the given issue.
func (c *cloudTryjobStore) deleteExpectationsForIssue(issueID int64) error {
	keys, _, err := c.getExpChangesForIssue(issueID, -1, -1, true)
	if err != nil {
		return err
	}

	// Delete all expectation entries and the expectation changes.
	var egroup errgroup.Group
	for _, expChangeKey := range keys {
		func(expChangeKey *datastore.Key) {
			egroup.Go(func() error {
				testDigestKeys, _, err := c.getTestDigestExps(expChangeKey, true)
				if err != nil {
					return err
				}
				if err := c.client.DeleteMulti(context.Background(), testDigestKeys); err != nil {
					return err
				}
				return nil
			})
		}(expChangeKey)
	}

	egroup.Go(func() error {
		return c.deleteExpChanges(keys)
	})

	return egroup.Wait()
}

// getExpChangesForIssue returns all the expectation changes for the given issue
// in revers chronological order. offset and size pick a subset of the result.
// Both are only considered if they are larger than 0. keysOnly indicates that we
// want keys only.
func (c *cloudTryjobStore) getExpChangesForIssue(issueID int64, offset, size int, keysOnly bool) ([]*datastore.Key, []*ExpChange, error) {
	q := ds.NewQuery(ds.TRYJOB_EXP_CHANGE).
		Filter("IssueID =", issueID).
		Filter("OK =", true).
		Order("TimeStamp")

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
func (c *cloudTryjobStore) getTestDigestExps(changeKey *datastore.Key, keysOnly bool) ([]*datastore.Key, []*TestDigestExp, error) {
	q := ds.NewQuery(ds.TEST_DIGEST_EXP).Ancestor(changeKey)
	if keysOnly {
		q = q.KeysOnly()
	}

	var exps []*TestDigestExp
	expsKeys, err := c.client.GetAll(context.Background(), q, &exps)
	if err != nil {
		return nil, nil, err
	}
	return expsKeys, exps, nil
}

// getTryjobsForIssue returns the tryjobs for the given issue.
func (c *cloudTryjobStore) getTryjobsForIssue(issueID int64, patchsetIDs []int64, keysOnly bool) ([]*datastore.Key, []*Tryjob, error) {
	if patchsetIDs == nil {
		patchsetIDs = []int64{-1}
	} else {
		sort.Slice(patchsetIDs, func(i, j int) bool { return patchsetIDs[i] < patchsetIDs[j] })
	}

	n := len(patchsetIDs)
	keysArr := make([][]*datastore.Key, n, n)
	valsArr := make([][]*Tryjob, n, n)
	resultSize := int32(0)
	var egroup errgroup.Group
	for idx, patchsetID := range patchsetIDs {
		func(idx int, patchsetID int64) {
			egroup.Go(func() error {
				query := ds.NewQuery(ds.TRYJOB).
					Filter("IssueID =", issueID)
				if patchsetID > 0 {
					query = query.Filter("PatchsetID =", patchsetID)
				}

				var tryjobs []*Tryjob = nil
				if keysOnly {
					query = query.KeysOnly()
				}

				keys, err := c.client.GetAll(context.Background(), query, &tryjobs)
				if err != nil {
					return fmt.Errorf("Error making GetAll call: %s", err)
				}
				keysArr[idx] = keys
				valsArr[idx] = tryjobs
				atomic.AddInt32(&resultSize, int32(len(keys)))
				return nil
			})
		}(idx, patchsetID)
	}

	if err := egroup.Wait(); err != nil {
		return nil, nil, err
	}

	retKeys := make([]*datastore.Key, 0, resultSize)
	var retVals []*Tryjob = nil
	if !keysOnly {
		retVals = make([]*Tryjob, 0, resultSize)
	}

	for idx, keys := range keysArr {
		retKeys = append(retKeys, keys...)
		if !keysOnly {
			retVals = append(retVals, valsArr[idx]...)
		}
	}

	return retKeys, retVals, nil
}

// updateIfNewer inserts the given item at the given key if the item is newer than what's in
// the database. Based on the return value of the newer function.
func (c *cloudTryjobStore) updateIfNewer(key *datastore.Key, item newerInterface) error {
	// Update the issue if the provided one is newer.
	updateFn := func(tx *datastore.Transaction) error {
		curr := reflect.New(reflect.TypeOf(item).Elem()).Interface()
		ok, err := c.getEntity(key, curr, tx)
		if err != nil {
			return err
		}

		if ok && !item.newer(curr) {
			return nil
		}

		if _, err := tx.Put(key, item); err != nil {
			return err
		}
		return nil
	}

	// Run the transaction.
	_, err := c.client.RunInTransaction(context.Background(), updateFn)
	return err
}

// getEntity loads the entity defined by key into target. If tx is not nil it uses the transaction.
func (c *cloudTryjobStore) getEntity(key *datastore.Key, target interface{}, tx *datastore.Transaction) (bool, error) {
	var err error
	if tx == nil {
		err = c.client.Get(context.Background(), key, target)
	} else {
		err = tx.Get(key, target)
	}

	if err != nil {
		// If we couldn't find it return nil, but no error.
		if err == datastore.ErrNoSuchEntity {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// getIssueKey returns a datastore key for the given issue id.
func (c *cloudTryjobStore) getIssueKey(id int64) *datastore.Key {
	ret := ds.NewKey(ds.ISSUE)
	ret.ID = id
	return ret
}

// getTryjobKey returns a datastore key for the given buildbucketID.
func (c *cloudTryjobStore) getTryjobKey(buildBucketID int64) *datastore.Key {
	ret := ds.NewKey(ds.TRYJOB)
	ret.ID = buildBucketID
	return ret
}

// getTryjobResultKey returns a key for the given tryjobResult.
func (c *cloudTryjobStore) getTryjobResultKey(tryjobKey *datastore.Key) *datastore.Key {
	ret := ds.NewKey(ds.TRYJOB_RESULT)
	ret.Parent = tryjobKey
	return ret
}
