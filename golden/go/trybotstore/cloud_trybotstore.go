package trybotstore

import (
	"context"
	"fmt"
	"reflect"
	"sort"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/sklog"
	"google.golang.org/api/option"
)

const (
	kind_Issue  = "issue"
	kind_Tryjob = "tryjob"
)

type cloudTrybotStore struct {
	client    *datastore.Client
	namespace string
}

func NewCloudTrybotStore(projectID, namespace, serviceAccountFile string) (TrybotStore, error) {
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
		_, tryjobs, err := c.getTryjobsForIssue(issueID, false)
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
					return tryjobs[i].Buildnumber < tryjobs[j].Buildnumber
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

	tryjobKeys, _, err := c.getTryjobsForIssue(issueID, true)
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

func (c *cloudTrybotStore) UpdateTryjob(issueID int64, tryjob *Tryjob) error {
	return c.updateIfNewer(c.getTryjobKey(tryjob.Buildnumber), tryjob)
}

func (c *cloudTrybotStore) getTryjobsForIssue(issueID int64, keysOnly bool) ([]*datastore.Key, []*Tryjob, error) {
	query := datastore.NewQuery(kind_Tryjob).
		Namespace(c.namespace).
		Filter("IssueID =", issueID)

	var ret []*Tryjob
	if keysOnly {
		query = query.KeysOnly()
	}

	keys, err := c.client.GetAll(context.Background(), query, &ret)
	return keys, ret, err
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

func (c *cloudTrybotStore) getTryjobKey(buildNumber int64) *datastore.Key {
	ret := datastore.IDKey(kind_Tryjob, buildNumber, nil)
	ret.Namespace = c.namespace
	return ret
}
