package trybotstore

import (
	"context"
	"reflect"

	"cloud.google.com/go/datastore"
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
	return nil, 0, nil
}

func (c *cloudTrybotStore) GetIssue(issueID int64, targetPatchsets []string) (*IssueDetails, error) {
	target := &IssueDetails{}
	if ok, err := c.getEntity(c.getIssueKey(issueID), target, nil); (err != nil) || !ok {
		return nil, err
	}
	return target, nil
}

func (c *cloudTrybotStore) UpdateIssue(details *IssueDetails) error {
	return c.updateIfNewer(c.getIssueKey(details.ID), details)
}

func (c *cloudTrybotStore) Delete(issueID int64) error {
	return c.client.Delete(context.Background(), c.getIssueKey(issueID))
}

func (c *cloudTrybotStore) UpdateTryjob(issueID int64, tryjob *Tryjob) error {
	return c.updateIfNewer(c.getTryjobKey(issueID, tryjob.Buildnumber), tryjob)
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

func (c *cloudTrybotStore) getTryjobKey(issueID, buildNumber int64) *datastore.Key {
	ret := datastore.IDKey(kind_Tryjob, buildNumber, c.getIssueKey(issueID))
	ret.Namespace = c.namespace
	return ret
}
