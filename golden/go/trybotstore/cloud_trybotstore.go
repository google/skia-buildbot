package trybotstore

import (
	"context"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/option"
)

const (
	kind_Issue  = "issue"
	kind_Trybot = "tryjob"
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
	ctx := context.Background()
	details := &IssueDetails{}
	if err := c.client.Get(ctx, c.getIssueKey(issueID), details); err != nil {
		// If we couldn't find it return nil, but no error.
		if err == datastore.ErrNoSuchEntity {
			return nil, nil
		}
		return nil, err
	}
	return details, nil
}

func (c *cloudTrybotStore) UpdateIssue(details *IssueDetails) error {
	ctx := context.Background()

	tx, err := c.client.NewTransaction(ctx)
	if err != nil {
		return err
	}

	issue, err := c.get(details.ID)

	if _, err := c.client.Put(ctx, c.getIssueKey(details.ID), details); err != nil {
		return err
	}
	return nil
}

func (c *cloudTrybotStore) Delete(issueID int64) error {
	return nil
}

func (c *cloudTrybotStore) AddTryjob(issueID, patchsetID int64, tryjob *Tryjob) error {
	return nil
}

func (c *cloudTrybotStore) getIssueKey(id int64) *datastore.Key {
	ret := datastore.IDKey(kind_Issue, id, nil)
	ret.Namespace = c.namespace
	return ret
}
