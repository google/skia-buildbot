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
			expCol.add(testName, digest, label.String(), kind, parent, &keys)
		}
	}

	return keys, expCol
}

func (e *ExpCollection) add(name, digest, label string, kind ds.Kind, parent *datastore.Key, keys *[]*datastore.Key) {
	curr := (*e)[len(*e)-1]
	if curr.full() {
		curr = newNameDigestLabels()
		*e = append(*e, curr)
		if keys != nil {
			*keys = append(*keys, ds.NewKeyWithParent(kind, parent))
		}
	}
	curr.add(name, digest, label)
}

func (e *ExpCollection) update(triagedDigests map[string]types.TestClassification) {
	// Make a copy of the changes to keep track of the ones we have already accounted for.
	change := (&Expectations{Tests: triagedDigests}).DeepCopy().Tests

	// empty keeps track of spots that have been changed to untriaged and can
	// be overridden. This avoids fragmentation of the batches of expecations.
	empty := []int{}
	newEntries := newNameDigestLabels()
	for batchIdx, exp := range *e {
		for idx, name := range exp.Names {
			digest := exp.Digests[idx]
			label := exp.Labels[idx]
			if newLabel, ok := change[name][digest]; ok {
				// Update the label and remove the entry.
				delete(change[name], digest)
				exp.Labels[idx] = newLabel.String()
				if newLabel == types.UNTRIAGED {
					empty = append(empty, batchIdx, idx)
				}
			} else {
				newEntries.add(name, digest, label)
			}
		}
	}

	emptyIdx := 0
	for idx, name := range newEntries.Names {
		digest := newEntries.Digests[idx]
		label := newEntries.Labels[idx]
		if emptyIdx < len(empty) {
			batch := (*e)[empty[emptyIdx]]
			idx := empty[emptyIdx+1]
			emptyIdx += 2
			batch.Names[idx] = name
			batch.Digests[idx] = digest
			batch.Labels[idx] = label
		} else {
			e.add(name, digest, label, "", nil, nil)
		}
	}
}

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

var (
	// The time delta that should guarantee that the data are consistent in ms.
	evConsistentDeltaMs int64 = 3600000
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
	idx := sort.Search(n, func(i int) bool { return m.TimeStamps[i] >= updated })
	if idx == n {
		m.TimeStamps = append(m.TimeStamps, updated)
		m.RecentChanges = append(m.RecentChanges, key)
	} else {
		m.TimeStamps = append(m.TimeStamps[0:idx], append([]int64{updated}, m.TimeStamps[idx:]...)...)
		m.RecentChanges = append(m.RecentChanges[0:idx], append([]*datastore.Key{key}, m.RecentChanges[idx:]...)...)
	}
}
