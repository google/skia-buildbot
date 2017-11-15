package bbtrybot

import "sync"

type TrybotStore interface {
	ListTrybotIssues(offset, size int) ([]*Issue, int, error)
	GetIssue(issueID int64, targetPatchsets []string) (*IssueDetails, error)
	Put(details *IssueDetails) error
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

func (m *MemTrybotStore) Put(newIssue *IssueDetails) error {
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
