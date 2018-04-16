package expstorage

import (
	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/ds"
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
	Key *datastore.Key `datastore:"__key__"`
	Exp [][3]string
}

func newNameDigestLabels() *NameDigestLabels {
	return &NameDigestLabels{
		Exp: make([][3]string, 0, nDigestsPerRec),
	}
}

func (e *NameDigestLabels) full() bool {
	return len(e.Exp) >= nDigestsPerRec
}

func (e *NameDigestLabels) add(name, digest, label string) {
	e.Exp = append(e.Exp, [3]string{name, digest, label})
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
	return nil
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

func (m *MatView) Update(key *datastore.Key, updated int64) {}
