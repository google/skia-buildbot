package trybotstore

import (
	"sync"
)

type TrybotStore interface {
	ListTrybotIssues(offset, size int) ([]*Issue, int, error)
	GetIssue(issueID int64, loadTryjobs bool, targetPatchsets []int64) (*IssueDetails, error)
	UpdateIssue(details *IssueDetails) error
	Delete(issueID int64) error
	GetTryjob(issueID, buildBucketID int64) (*Tryjob, error)
	GetTryjobResults(issueID int64, patchsetID []int64) ([]*Tryjob, [][]*TryjobResult, error)
	UpdateTryjob(issueID int64, tryjob *Tryjob) error
	UpdateTryjobResult(tryjob *Tryjob, results []*TryjobResult) error
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

func (m *MemTrybotStore) GetIssue(issueID int64, loadTryjobs bool, targetPatchsets []int64) (*IssueDetails, error) {
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

func (m *MemTrybotStore) UpdateTryjob(issueID int64, tryjob *Tryjob) error {
	return nil
}

func (m *MemTrybotStore) UpdateTryjobResult(tryjob *Tryjob, result []*TryjobResult) error {
	return nil
}

func (m *MemTrybotStore) GetTryjob(issueID, buildBucketID int64) (*Tryjob, error) {
	return nil, nil
}

func (m *MemTrybotStore) GetTryjobResults(issueID int64, patchsetID []int64) ([]*Tryjob, [][]*TryjobResult, error) {
	return nil, nil, nil
}
