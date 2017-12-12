package tryjobstore

import (
	"sync"

	"go.skia.org/infra/golden/go/expstorage"
	"go.skia.org/infra/golden/go/types"
)

type MemTryjobStore struct {
	issues []*IssueDetails
	mutex  sync.Mutex
}

func NewMemTryjobStore() TryjobStore {
	return &MemTryjobStore{
		issues: []*IssueDetails{},
	}
}

func (m *MemTryjobStore) ListIssues() ([]*Issue, int, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	ret := make([]*Issue, 0, len(m.issues))
	for _, issue := range m.issues {
		ret = append(ret, issue.Issue)
	}
	return ret, len(m.issues), nil
}

func (m *MemTryjobStore) GetIssue(issueID int64, loadTryjobs bool, targetPatchsets []int64) (*IssueDetails, error) {
	m.mutex.Lock()
	defer m.mutex.Unlock()
	for _, issue := range m.issues {
		if issue.ID == issueID {
			return issue, nil
		}
	}
	return nil, nil
}

func (m *MemTryjobStore) UpdateIssue(newIssue *IssueDetails) error {
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

func (m *MemTryjobStore) DeleteIssue(issueID int64) error {
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

func (m *MemTryjobStore) UpdateTryjob(issueID int64, tryjob *Tryjob) error {
	return nil
}

func (m *MemTryjobStore) UpdateTryjobResult(tryjob *Tryjob, result []*TryjobResult) error {
	return nil
}

func (m *MemTryjobStore) GetTryjob(issueID, buildBucketID int64) (*Tryjob, error) {
	return nil, nil
}

func (m *MemTryjobStore) GetTryjobResults(issueID int64, patchsetID []int64) ([]*Tryjob, [][]*TryjobResult, error) {
	return nil, nil, nil
}

func (m *MemTryjobStore) AddChange(issueID int64, changes map[string]types.TestClassification, userId string) error {
	return nil
}

func (m *MemTryjobStore) GetExpectations(issueID int64) (exp *expstorage.Expectations, err error) {
	return nil, nil
}

func (m *MemTryjobStore) UndoChange(issueID int64, changeID int64, userID string) (map[string]types.TestClassification, error) {
	return nil, nil
}

func (m *MemTryjobStore) QueryLog(issueID int64, offset, size int, details bool) ([]*expstorage.TriageLogEntry, int, error) {
	return nil, 0, nil
}
