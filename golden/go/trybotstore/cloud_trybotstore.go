package trybotstore

import (
	"context"
	"fmt"
	"reflect"
	"sort"
	"sync/atomic"

	"github.com/davecgh/go-spew/spew"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/sync/errgroup"
	"google.golang.org/api/option"
)

const (
	kind_Issue        = "issue"
	kind_Tryjob       = "tryjob"
	kind_TryjobResult = "tryjob_result"
)

type cloudTrybotStore struct {
	client    *datastore.Client
	namespace string
}

func NewCloudTrybotStore(projectID, namespace string, serviceAccountFile string) (TrybotStore, error) {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, projectID, option.WithServiceAccountFile(serviceAccountFile))
	if err != nil {
		return nil, err
	}

	return &cloudTrybotStore{
		client:    client,
		namespace: namespace,
	}, nil
}

func (c *cloudTrybotStore) ListTrybotIssues(offset, size int) ([]*Issue, int, error) {
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

func (c *cloudTrybotStore) GetIssue(issueID int64, loadTryjobs bool, targetPatchsets []int64) (*IssueDetails, error) {
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

func (c *cloudTrybotStore) UpdateIssue(details *IssueDetails) error {
	return c.updateIfNewer(c.getIssueKey(details.ID), details)
}

func (c *cloudTrybotStore) Delete(issueID int64) error {
	ctx := context.Background()
	key := c.getIssueKey(issueID)
	curr := &IssueDetails{}
	ok, err := c.getEntity(key, curr, nil)
	if err != nil {
		return err
	}

	// If there is no entity we are done.
	if !ok {
		return nil
	}

	tryjobKeys, _, err := c.getTryjobsForIssue(issueID, nil, true)
	if err != nil {
		return fmt.Errorf("Error retrieving tryjob keys: %s", err)
	}

	if len(tryjobKeys) > 0 {
		// Delete the tryjobs.
		if err := c.client.DeleteMulti(ctx, tryjobKeys); err != nil {
			return fmt.Errorf("Error deleting %d tryjobs for issue %d: %s", len(tryjobKeys), issueID, err)
		}
		sklog.Infof("Deleted %d for issue %d", len(tryjobKeys), issueID)
	}

	// Delete the entity.
	return c.client.Delete(ctx, key)
}

func (c *cloudTrybotStore) GetTryjob(issueID, buildBucketID int64) (*Tryjob, error) {
	ret := &Tryjob{}
	if err := c.client.Get(context.Background(), c.getTryjobKey(buildBucketID), ret); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, nil
		}
		return nil, err
	}

	return ret, nil
}

func (c *cloudTrybotStore) UpdateTryjob(issueID int64, tryjob *Tryjob) error {
	return c.updateIfNewer(c.getTryjobKey(tryjob.BuildBucketID), tryjob)
}

func (c *cloudTrybotStore) GetTryjobResults(issueID int64, patchsetIDs []int64) ([]*Tryjob, [][]*TryjobResult, error) {
	tryjobKeys, tryjobs, err := c.getTryjobsForIssue(issueID, patchsetIDs, false)
	if err != nil {
		return nil, nil, err
	}

	sklog.Info("TRYJOBS: %s", spew.Sdump(tryjobs))

	// Collect all results across patchsets.
	tryjobResults := make([][]*TryjobResult, len(tryjobKeys), len(tryjobKeys))
	ctx := context.Background()
	var egroup errgroup.Group
	for idx, key := range tryjobKeys {
		func(idx int, key *datastore.Key) {
			egroup.Go(func() error {
				query := datastore.NewQuery(kind_TryjobResult).
					Namespace(c.namespace).
					Ancestor(key)
				queryResult := []*TryjobResult{}
				if _, err := c.client.GetAll(ctx, query, &queryResult); err != nil {
					return err
				}
				tryjobResults[idx] = queryResult
				return nil
			})
		}(idx, key)
	}

	if err := egroup.Wait(); err != nil {
		return nil, nil, fmt.Errorf("Error getting tryjob results: %s", err)
	}

	return tryjobs, tryjobResults, nil
}

func (c *cloudTrybotStore) UpdateTryjobResult(tryjob *Tryjob, results []*TryjobResult) error {
	tryjobKey := c.getTryjobKey(tryjob.BuildBucketID)
	keys := make([]*datastore.Key, 0, len(results))
	for _, result := range results {
		keys = append(keys, c.getTryjobResultKey(tryjobKey, result.Digest))
	}

	const batchSize = 300
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

func (c *cloudTrybotStore) getTryjobsForIssue(issueID int64, patchsetIDs []int64, keysOnly bool) ([]*datastore.Key, []*Tryjob, error) {
	if patchsetIDs == nil {
		patchsetIDs = []int64{-1}
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

				if patchsetID > 0 {
					query = query.Filter("PatchsetID =", patchsetID)
				}

				var tryjobs []*Tryjob = nil
				if keysOnly {
					query = query.KeysOnly()
				}

				keys, err := c.client.GetAll(context.Background(), query, &tryjobs)
				if err != nil {
					return fmt.Errorf("Error retrieving %s", err)
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

func (c *cloudTrybotStore) updateIfNewer(key *datastore.Key, item newerInterface) error {
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

func (c *cloudTrybotStore) getEntity(key *datastore.Key, target interface{}, tx *datastore.Transaction) (bool, error) {
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

func (c *cloudTrybotStore) getIssueKey(id int64) *datastore.Key {
	ret := datastore.IDKey(kind_Issue, id, nil)
	ret.Namespace = c.namespace
	return ret
}

func (c *cloudTrybotStore) getTryjobKey(buildBucketID int64) *datastore.Key {
	ret := datastore.IDKey(kind_Tryjob, buildBucketID, nil)
	ret.Namespace = c.namespace
	return ret
}

func (c *cloudTrybotStore) getTryjobResultKey(tryjobKey *datastore.Key, keyStr string) *datastore.Key {
	ret := datastore.NameKey(kind_TryjobResult, keyStr, tryjobKey)
	ret.Namespace = c.namespace
	return ret
}
