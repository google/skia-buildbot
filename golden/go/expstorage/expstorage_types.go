package expstorage

import (
	"sort"
	"time"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

const nDigestsPerRec = 2000

// ExpChange is used to store an expectation change in the database. Each
// expecation change is an atomic change to expectations for an issue.
// The actualy expecations are captured in instances of TestDigestExp.
type ExpChange struct {
	ChangeID     *datastore.Key `datastore:"__key__"`
	IssueID      int64
	UserID       string
	TimeStamp    int64
	Count        int64
	UndoChangeID int64
	OK           bool
}

// TestDigestExp is used to store expectations for an issue in the database.
// Each entity is a child of instance of ExpChange. It captures the expectation
// of one Test/Digest pair.
type NameDigestLabels struct {
	Key     *datastore.Key `datastore:"__key__"`
	Names   []string
	Digests []string
	Labels  []string
}

func newNameDigestLabels() *NameDigestLabels {
	return &NameDigestLabels{
		Names:   make([]string, 0, nDigestsPerRec),
		Digests: make([]string, 0, nDigestsPerRec),
		Labels:  make([]string, 0, nDigestsPerRec),
	}
}

func (e *NameDigestLabels) full() bool {
	return len(e.Names) >= nDigestsPerRec
}

func (e *NameDigestLabels) add(name, digest, label string) {
	e.Names = append(e.Names, name)
	e.Digests = append(e.Digests, digest)
	e.Labels = append(e.Labels, label)
}

type ExpCollection []*NameDigestLabels

func buildExpCollection(changes map[string]types.TestClassification, kind ds.Kind, parent *datastore.Key) ([]*datastore.Key, ExpCollection) {
	expCol := ExpCollection{newNameDigestLabels()}
	keys := []*datastore.Key{ds.NewKeyWithParent(kind, parent)}

	// Assemble the collection of expectations.
	for testName, classification := range changes {
		for digest, label := range classification {
			curr := expCol[len(expCol)-1]
			if curr.full() {
				curr = newNameDigestLabels()
				expCol = append(expCol, curr)
				keys = append(keys, ds.NewKeyWithParent(kind, parent))
			}
			curr.add(testName, digest, label.String())
		}
	}

	return keys, expCol
}

func (e *ExpCollection) update(change map[string]types.TestClassification) {}

func (e ExpCollection) toExpectations() *Expectations {
	ret := map[string]types.TestClassification{}
	for _, exp := range e {
		for idx, name := range exp.Names {
			digest := exp.Digests[idx]
			label := types.LabelFromString(exp.Labels[idx])
			testEntry, ok := ret[name]
			if !ok {
				ret[name] = types.TestClassification{digest: label}
			} else {
				testEntry[digest] = label
			}
		}
	}
	return &Expectations{
		Tests: ret,
	}
}

func (e ExpCollection) getKeys(kind ds.Kind, parentKey *datastore.Key) ([]*datastore.Key, error) {
	ret := make([]*datastore.Key, len(e))
	for idx, entry := range e {
		ret[idx] = entry.Key
		if ret[idx] == nil {
			ret[idx] = ds.NewKeyWithParent(kind, parentKey)
		}
	}
	return ret, nil
}

type MatView struct {
	RecentChanges []*datastore.Key
	TimeStamps    []int64
}

func emptyMatView() *MatView {
	return &MatView{
		RecentChanges: []*datastore.Key{},
		TimeStamps:    []int64{},
	}
}

const (
	// The time delta that should guarantee that the data are consistent in ms.
	evConsistentDeltaMs = 3600000
)

func (m *MatView) Update(key *datastore.Key, updated int64) {
	// Filter out the existing
	nowMs := util.TimeStamp(time.Millisecond)
	firstIndex := len(m.TimeStamps)
	for idx, ts := range m.TimeStamps {
		if (nowMs - ts) <= evConsistentDeltaMs {
			firstIndex = idx
			break
		}
	}
	m.TimeStamps = append([]int64(nil), m.TimeStamps[firstIndex:]...)
	m.RecentChanges = append([]*datastore.Key(nil), m.RecentChanges[firstIndex:]...)

	n := len(m.TimeStamps)
	idx := sort.Search(n, func(i int) bool { return m.TimeStamps[i] > updated })
	if idx == n {
		m.TimeStamps = append(m.TimeStamps, updated)
		m.RecentChanges = append(m.RecentChanges, key)
	} else {
		m.TimeStamps = append(m.TimeStamps[0:idx+1], append([]int64{updated}, m.TimeStamps[idx+1:]...)...)
		m.RecentChanges = append(m.RecentChanges[0:idx+1], append([]*datastore.Key{key}, m.RecentChanges[idx+1:]...)...)
	}
}
