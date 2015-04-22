package digeststore

import (
	"sync"

	ptypes "go.skia.org/infra/perf/go/types"
)

// DigestInfo aggregates all information we have about an individual digest.
type DigestInfo struct {
	// TestName for this digest.
	TestName string

	// Digest that uniquely identifies the digest within this test.
	Digest string

	// First containes the timestamp of the first occurance of this digest.
	First int64

	// Last contains the timestamp of the last time we have seen this digest.
	Last int64

	// Exception stores a string representing the exception that was encountered
	// retrieving this digest. If Exception is "" then there was no problem.
	Exception string

	// IssueIDs is a list of issue ids that are associated with this digest.
	IssueIDs []int
}

type DigestStore interface {
	// GetDigestInfo returns the information about the given testName-digest
	// pair.
	GetDigestInfo(testName, digest string) (*DigestInfo, bool, error)

	// UpdateDigestTimeStamps updates the information about the digest. If there
	// is no "new" information it will not change the underlying datastore.
	UpdateDigestTimeStamps(testName, digest string, commit *ptypes.Commit) (*DigestInfo, error)
}

// MemDigestStore implements the DigestStore interface in memory.
type MemDigestStore struct {
	digestInfos map[string]map[string]*DigestInfo
	readCopy    map[string]map[string]*DigestInfo
	readMutex   sync.RWMutex
	updateMutex sync.Mutex
}

func NewMemDigestStore() DigestStore {
	return &MemDigestStore{
		digestInfos: map[string]map[string]*DigestInfo{},
		readCopy:    map[string]map[string]*DigestInfo{},
	}
}

func (m *MemDigestStore) GetDigestInfo(testName, digest string) (*DigestInfo, bool, error) {
	m.readMutex.RLock()
	defer m.readMutex.RUnlock()
	ret, ok := m.readCopy[testName][digest]
	return ret, ok, nil
}

func (m *MemDigestStore) UpdateDigestTimeStamps(testName, digest string, commit *ptypes.Commit) (*DigestInfo, error) {
	m.updateMutex.Lock()
	defer m.updateMutex.Unlock()

	updated := true
	if curr, ok := m.digestInfos[testName][digest]; ok {
		if curr.Last < commit.CommitTime {
			curr.Last = commit.CommitTime
		} else if curr.First > commit.CommitTime {
			curr.First = commit.CommitTime
		} else {
			updated = false
		}
	} else {
		if _, ok = m.digestInfos[testName]; !ok {
			m.digestInfos[testName] = map[string]*DigestInfo{}
		}
		m.digestInfos[testName][digest] = &DigestInfo{
			TestName: testName,
			Digest:   digest,
			First:    commit.CommitTime,
			Last:     commit.CommitTime,
			IssueIDs: []int{},
		}
	}

	if updated {
		m.makeReadCopy()
	}

	ret, _, err := m.GetDigestInfo(testName, digest)
	return ret, err
}

func (m *MemDigestStore) makeReadCopy() {
	newReadCopy := make(map[string]map[string]*DigestInfo, len(m.digestInfos))
	for testName, digests := range m.digestInfos {
		newReadCopy[testName] = make(map[string]*DigestInfo, len(digests))
		for digest, digestInfo := range digests {
			temp := &DigestInfo{}
			*temp = *digestInfo
			newReadCopy[testName][digest] = temp
		}
	}

	m.readMutex.Lock()
	m.readCopy = newReadCopy
	m.readMutex.Unlock()
}
