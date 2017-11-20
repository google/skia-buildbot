package trybotstore

import (
	"context"
	"sync"

	"cloud.google.com/go/datastore"
	"google.golang.org/api/option"
)

type TrybotStore interface {
	ListTrybotIssues(offset, size int) ([]*Issue, int, error)
	GetIssue(issueID int64, targetPatchsets []string) (*IssueDetails, error)
	UpdateIssue(details *IssueDetails) error
	Delete(issueID int64) error
	AddTryjob(issueID, patchsetID int64, tryjob *Tryjob) error
}

type MemTrybotStore struct {
	issues []*IssueDetails
	mutex  sync.Mutex
}

func NewMemTrybotStore() TrybotStore {
	return &MemTrybotStore{
		issues: []*IssueDetails{},
	}
}

func (m *MemTrybotStore) ListTrybotIssues(offset, size int) ([]*Issue, int, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	ret := make([]*Issue, 0, len(m.issues))
	for _, issue := range m.issues {
		ret = append(ret, issue.Issue)
	}
	return ret, len(m.issues), nil
}

func (m *MemTrybotStore) GetIssue(issueID int64, targetPatchsets []string) (*IssueDetails, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, issue := range m.issues {
		if issue.ID == issueID {
			return issue, nil
		}
	}
	return nil, nil
}

func (m *MemTrybotStore) UpdateIssue(newIssue *IssueDetails) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for idx, issue := range m.issues {
		if issue.ID == newIssue.ID {
			*m.issues[idx] = *newIssue
			return nil
		}
	}

	m.issues = append(m.issues, newIssue)
	return nil
}

func (m *MemTrybotStore) Delete(issueID int64) error {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for idx, issue := range m.issues {
		if issue.ID == issueID {
			m.issues = append(m.issues[:idx], m.issues[idx+1:]...)
			break
		}
	}

	return nil
}

func (m *MemTrybotStore) AddTryjob(issueID, patchsetID int64, tryjob *Tryjob) error {
	return nil
}

type cloudTrybotStore struct {
	client *datastore.Client
}

func NewCloudTrybotStore(projectID string, serviceAccountFile string) (TrybotStore, error) {
	ctx := context.Background()
	client, err := datastore.NewClient(ctx, projectID, option.WithServiceAccountFile(serviceAccountFile))
	if err != nil {
		return nil, err
	}

	return &cloudTrybotStore{
		client: client,
	}, nil
}

func (c *cloudTrybotStore) ListTrybotIssues(offset, size int) ([]*Issue, int, error) {
	return nil, 0, nil
}

func (c *cloudTrybotStore) GetIssue(issueID int64, targetPatchsets []string) (*IssueDetails, error) {
	return nil, nil
}

func (c *cloudTrybotStore) UpdateIssue(details *IssueDetails) error {
	if details.clean {
		return nil
	}

	return nil
}

func (c *cloudTrybotStore) Delete(issueID int64) error {
	return nil
}

func (c *cloudTrybotStore) AddTryjob(issueID, patchsetID int64, tryjob *Tryjob) error {
	return nil
}
