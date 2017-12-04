package tryjobstore

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync/atomic"
	"time"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"
)

type TryjobStore interface {
	ListIssues(offset, size int) ([]*Issue, int, error)
	GetIssue(issueID int64, loadTryjobs bool, targetPatchsets []int64) (*IssueDetails, error)
	UpdateIssue(details *IssueDetails) error
	DeleteIssue(issueID int64) error
	GetTryjob(issueID, buildBucketID int64) (*Tryjob, error)
	GetTryjobResults(issueID int64, patchsetID []int64) ([]*Tryjob, [][]*TryjobResult, error)
	UpdateTryjob(issueID int64, tryjob *Tryjob) error
	UpdateTryjobResult(tryjob *Tryjob, results []*TryjobResult) error
	AddChange(issueID int64, changes map[string]types.TestClassification, userId string) error
	GetExpectations(issueID int64) (exp *expstorage.Expectations, err error)
	UndoChange(issueID int64, changeID int64, userID string) (map[string]types.TestClassification, error)
	QueryLog(offset, size int, details bool) ([]*expstorage.TriageLogEntry, int, error)
}

const (
	kind_Issue         = "Issue"
	kind_Tryjob        = "Tryjob"
	kind_TryjobResult  = "TryjobResult"
	kind_ExpChange     = "ExpChange"
	kind_TestDigestExp = "TestDigestExp"
)

const (
	batchSize = 300
)

type cloudTryjobStore struct {
	client    *datastore.Client
	namespace string
}

func NewCloudTryjobStore(projectID, namespace string, serviceAccountFile string) (TryjobStore, error) {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, projectID, option.WithServiceAccountFile(serviceAccountFile))
	if err != nil {
		return nil, err
	}

	return &cloudTryjobStore{
		client:    client,
		namespace: namespace,
	}, nil
}

func (c *cloudTryjobStore) ListIssues(offset, size int) ([]*Issue, int, error) {
	query := datastore.NewQuery(kind_Issue).Namespace(c.namespace).KeysOnly()
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

func (c *cloudTryjobStore) GetIssue(issueID int64, loadTryjobs bool, targetPatchsets []int64) (*IssueDetails, error) {
	target := &IssueDetails{}
	key := c.getIssueKey(issueID)
	if ok, err := c.getEntity(key, target, nil); (err != nil) || !ok {
		return nil, err
	}

	if loadTryjobs {
		_, tryjobs, err := c.getTryjobsForIssue(issueID, nil, false)
		if err != nil {
			return nil, err
		}

		for _, tj := range tryjobs {
			ps, _ := target.findPatchset(tj.PatchsetID)
			if ps == nil {
				return nil, fmt.Errorf("Unable to find patchset %d in issue %d:", tj.PatchsetID, target.ID)
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

func (c *cloudTryjobStore) UpdateIssue(details *IssueDetails) error {
	return c.updateIfNewer(c.getIssueKey(details.ID), details)
}

func (c *cloudTryjobStore) DeleteIssue(issueID int64) error {
	ctx := context.Background()
	key := c.getIssueKey(issueID)

	// Delete any tryjobs that are still there.
	if err := c.deleteTryjobsForIssue(issueID); err != nil {
		return err
	}

	// Delete all expectations for this issue.
	if err := c.deleteExpectationsForIssue(issueID); err != nil {
		return err
	}

	// Delete the entity.
	return c.client.Delete(ctx, key)
}

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

func (c *cloudTryjobStore) UpdateTryjob(issueID int64, tryjob *Tryjob) error {
	return c.updateIfNewer(c.getTryjobKey(tryjob.BuildBucketID), tryjob)
}

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
	sklog.Infof("Deleted %d for issue %d", len(tryjobKeys), issueID)

	return nil
}

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
				query := datastore.NewQuery(kind_TryjobResult).
					Namespace(c.namespace).
					Ancestor(key)

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

func (c *cloudTryjobStore) GetTryjobResults(issueID int64, patchsetIDs []int64) ([]*Tryjob, [][]*TryjobResult, error) {
	tryjobKeys, tryjobs, err := c.getTryjobsForIssue(issueID, patchsetIDs, false)
	if err != nil {
		return nil, nil, err
	}

	_, tryjobResults, err := c.getResultsForTryjobs(tryjobKeys, false)
	if err != nil {
		return nil, nil, err
	}

	return tryjobs, tryjobResults, nil
}

func (c *cloudTryjobStore) UpdateTryjobResult(tryjob *Tryjob, results []*TryjobResult) error {
	tryjobKey := c.getTryjobKey(tryjob.BuildBucketID)
	keys := make([]*datastore.Key, 0, len(results))
	for _, result := range results {
		keys = append(keys, c.getTryjobResultKey(tryjobKey, result.Digest))
	}

	// var egroup errgroup.Group
	for i := 0; i < len(keys); i += batchSize {
		endIdx := util.MinInt(i+batchSize, len(keys))

		if _, err := c.client.PutMulti(context.Background(), keys[i:endIdx], results[i:endIdx]); err != nil {
			return err
		}

		// func(keys []*datastore.Key, results []*TryjobResult) {
		// 	egroup.Go(func() error {
		// 		_, err := c.client.PutMulti(context.Background(), keys, results)
		// 		return err
		// 	})
		// }(keys[i:endIdx], results[i:endIdx])
	}

	// if err := egroup.Wait(); err != nil {
	// 	return fmt.Errorf("Error updating tryjob results: %s", err)
	// }

	return nil
}

func (c *cloudTryjobStore) AddChange(issueID int64, changes map[string]types.TestClassification, userID string) (err error) {
	// Write the change record.
	ctx := context.Background()
	expChange := &ExpChange{
		IssueID:   issueID,
		UserID:    userID,
		TimeStamp: util.TimeStamp(time.Millisecond),
	}

	var changeKey *datastore.Key
	if changeKey, err = c.client.Put(ctx, c.getIncompleteKey(kind_ExpChange, nil), expChange); err != nil {
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
		tdeKeys[idx] = c.getIncompleteKey(kind_TestDigestExp, changeKey)
	}

	if _, err = c.client.PutMulti(ctx, tdeKeys, testChanges); err != nil {
		return err
	}

	// Mark the expectation change as valid.
	expChange.OK = true
	if _, err = c.client.Put(ctx, changeKey, expChange); err != nil {
		return err
	}

	return nil
}

// deleteExpChange cleans up an expectations change that failed.
func (c *cloudTryjobStore) deleteExpChanges(keys []*datastore.Key) error {
	return c.client.DeleteMulti(context.Background(), keys)
}

func (c *cloudTryjobStore) deleteExpectationsForIssue(issueID int64) error {
	keys, err := c.getExpChangesForIssue(issueID)
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

func (c *cloudTryjobStore) getExpChangesForIssue(issueID int64) ([]*datastore.Key, error) {
	expChangeQuery := datastore.NewQuery(kind_ExpChange).
		Namespace(c.namespace).
		Filter("IssueID =", issueID).
		Filter("OK =", true).
		Order("TimeStamp").KeysOnly()

	return c.client.GetAll(context.Background(), expChangeQuery, nil)
}

func (c *cloudTryjobStore) getTestDigestExps(changeKey *datastore.Key, keysOnly bool) ([]*datastore.Key, []*TestDigestExp, error) {
	q := datastore.NewQuery(kind_TestDigestExp).
		Namespace(c.namespace).
		Ancestor(changeKey)

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

func (c *cloudTryjobStore) GetExpectations(issueID int64) (exp *expstorage.Expectations, err error) {
	// Get all expectation changes and iterate over them updating the result.
	expChangeKeys, err := c.getExpChangesForIssue(issueID)
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
				ret.Add(oneChange.Name, oneChange.Digest, types.LabelFromString(oneChange.Label))
			}
		}
	}

	return ret, nil
}

// TODO(stephana): This needs to be implemented or removed.
func (c *cloudTryjobStore) UndoChange(issueID int64, changeID int64, userID string) (map[string]types.TestClassification, error) {
	return nil, nil
}

// TODO(stephana): This needs to be implemented or removed.
func (c *cloudTryjobStore) QueryLog(offset, size int, details bool) ([]*expstorage.TriageLogEntry, int, error) {
	return nil, 0, nil
}

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
				query := datastore.NewQuery(kind_Tryjob).
					Namespace(c.namespace).
					Filter("IssueID =", issueID)
					// Order("BuildBucketID")

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

func (c *cloudTryjobStore) getIssueKey(id int64) *datastore.Key {
	ret := datastore.IDKey(kind_Issue, id, nil)
	ret.Namespace = c.namespace
	return ret
}

func (c *cloudTryjobStore) getTryjobKey(buildBucketID int64) *datastore.Key {
	ret := datastore.IDKey(kind_Tryjob, buildBucketID, nil)
	ret.Namespace = c.namespace
	return ret
}

func (c *cloudTryjobStore) getTryjobResultKey(tryjobKey *datastore.Key, keyStr string) *datastore.Key {
	ret := datastore.NameKey(kind_TryjobResult, keyStr, tryjobKey)
	ret.Namespace = c.namespace
	return ret
}

func (c *cloudTryjobStore) getIncompleteKey(kind string, ancenstor *datastore.Key) *datastore.Key {
	ret := datastore.IncompleteKey(kind, ancenstor)
	ret.Namespace = c.namespace
	return ret
}
