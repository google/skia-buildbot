package expstorage

import (
	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/golden/go/types"
)

// ExpChange is used to store an expectation change in the database. Each
// expecation change is an atomic change to expectations for an issue.
// The actualy expecations are captured in instances of TestDigestExp.
type ExpChange struct {
	ChangeID     *datastore.Key `datastore:"__key__"`
	IssueID      int64
	UserID       string
	TimeStamp    int64 `datastore:",noindex"`
	Count        int64 `datastore:",noindex"`
	UndoChangeID int64
	OK           bool
}

// nDigestsPerRec is the number of (Testname, Digest, Label) tripples we store
// in a single instance of TestDigestExp. The value is chosen to reliable fit
// into the space limits of a datastore instance.
const nDigestsPerRec = 2000

// TestDigestExp is used to store expectations for an issue in the database.
// It stores nDigestsPerRec expectations in each entity so we can retrieve
// many expectations at once.
type TestDigestExp struct {
	Key     *datastore.Key `datastore:"__key__"` // Key is populated when the entity is loaded.
	Names   []string
	Digests []string
	Labels  []string
}

// newTestDigestExp allocates a new block for hold expecations.
func newTestDigestExp() *TestDigestExp {
	return &TestDigestExp{
		Names:   make([]string, 0, nDigestsPerRec),
		Digests: make([]string, 0, nDigestsPerRec),
		Labels:  make([]string, 0, nDigestsPerRec),
	}
}

// full returns true if this block of expectations is at it's limits and a new
// new should be allocated.
func (e *TestDigestExp) full() bool {
	return len(e.Names) >= nDigestsPerRec
}

// add addes an new triple to the expectations. It does not check whether the
// current block is full.
func (e *TestDigestExp) add(name, digest, label string) {
	e.Names = append(e.Names, name)
	e.Digests = append(e.Digests, digest)
	e.Labels = append(e.Labels, label)
}

// TDESlice is a slice of TestDigestExp allowing to store an abitrary number of
// expectations in multiple blocks.
type TDESlice []*TestDigestExp

func (e TDESlice) empty() bool {
	return (len(e) == 0) || (len(e[0].Names) == 0)
}

// buildTDESlice converts the given expecectation(change)s to a TDESlice instance
// for storage in the cloud datastore.
func buildTDESlice(expChange map[string]types.TestClassification, kind ds.Kind, parent *datastore.Key) ([]*datastore.Key, TDESlice) {
	expCol := TDESlice{newTestDigestExp()}
	keys := []*datastore.Key{ds.NewKeyWithParent(kind, parent)}

	// Assemble the collection of expectations.
	for testName, classification := range expChange {
		for digest, label := range classification {
			expCol.add(testName, digest, label.String(), kind, parent, &keys)
		}
	}

	return keys, expCol
}

// add adds a new expectation to the current TDESlice. The given entity kind
func (e *TDESlice) add(name, digest, label string, kind ds.Kind, parent *datastore.Key, keys *[]*datastore.Key) {
	curr := (*e)[len(*e)-1]
	if curr.full() {
		curr = newTestDigestExp()
		*e = append(*e, curr)
		if keys != nil {
			*keys = append(*keys, ds.NewKeyWithParent(kind, parent))
		}
	}
	curr.add(name, digest, label)
}

// update the existing collection of expectations.
func (e *TDESlice) update(triagedDigests map[string]types.TestClassification) {
	// If the collection is empty then just build a new one.
	if len(*e) == 0 {
		_, *e = buildTDESlice(triagedDigests, "", nil)
		return
	}

	// Make a copy of the changes to keep track of the ones we have already accounted for.
	change := (&Expectations{Tests: triagedDigests}).DeepCopy().Tests

	// empty keeps track of spots that have been changed to untriaged and can
	// be overridden. This avoids fragmentation of the batches of expecations.
	empty := []int{}
	untriagedStr := types.UNTRIAGED.String()

	for batchIdx, exp := range *e {
		for idx, name := range exp.Names {
			digest := exp.Digests[idx]
			if newLabel, ok := change[name][digest]; ok {
				// Update the label and remove the entry.
				exp.Labels[idx] = newLabel.String()
				delete(change[name], digest)
			}
			// Mark untriaged as empty slots for new entries to avoid fragmentation.
			if exp.Labels[idx] == untriagedStr {
				empty = append(empty, batchIdx, idx)
			}
		}
	}

	emptyIdx := 0
	for name, digests := range change {
		for digest, label := range digests {
			// If we still have empty slots then insert this expectation.
			if emptyIdx < len(empty) {
				batch := (*e)[empty[emptyIdx]]
				idx := empty[emptyIdx+1]
				emptyIdx += 2
				batch.Names[idx] = name
				batch.Digests[idx] = digest
				batch.Labels[idx] = label.String()
			} else {
				e.add(name, digest, label.String(), "", nil, nil)
			}
		}
	}
}

// convert the expectations to the datastructure that is easier for lookup.
func (e TDESlice) toExpectations(filterUntriaged bool) *Expectations {
	ret := map[string]types.TestClassification{}
	for _, exp := range e {
		for idx, name := range exp.Names {
			digest := exp.Digests[idx]
			label := types.LabelFromString(exp.Labels[idx])
			if filterUntriaged && (label == types.UNTRIAGED) {
				continue
			}

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

// getKeys returns the keys for this collection of expectations. If an instance
// of TestDigestExp does not contain a key (because it was not loaded from the
// datastore) we create a new key.
func (e TDESlice) getKeys(kind ds.Kind, parentKey *datastore.Key) ([]*datastore.Key, error) {
	ret := make([]*datastore.Key, len(e))
	for idx, entry := range e {
		ret[idx] = entry.Key
		if ret[idx] == nil {
			ret[idx] = ds.NewKeyWithParent(kind, parentKey)
		}
	}
	return ret, nil
}

// EventExpectationChange is the structure that is sent in expectation change events.
type EventExpectationChange struct {
	IssueID     int64
	TestChanges map[string]types.TestClassification
}

// evExpChange creates a new instance of EventExptationChange.
func evExpChange(changes map[string]types.TestClassification, issueID int64) *EventExpectationChange {
	return &EventExpectationChange{
		TestChanges: changes,
		IssueID:     issueID,
	}
}
