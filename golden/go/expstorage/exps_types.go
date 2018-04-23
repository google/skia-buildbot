package expstorage

import (
	"math/rand"
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
	TimeStamp    int64 `datastore:",noindex"`
	Count        int64 `datastore:",noindex"`
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
	// If the collection is empty then just build a new one.
	if len(*e) == 0 {
		_, *e = buildExpCollection(triagedDigests, "", nil)
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
	RecentChanges []*datastore.Key `datastore:",noindex"`
}

func emptyMatView() *MatView {
	return &MatView{
		RecentChanges: []*datastore.Key{},
	}
}

var (
	// The time delta that should guarantee that the data are consistent in ms.
	evConsistentDeltaMs int64 = 3600000
)

func (m *MatView) Update(key *datastore.Key) {
	// Filter out the existing
	nowMs := util.TimeStamp(time.Millisecond)
	lastIndex := len(m.RecentChanges)
	for idx, key := range m.RecentChanges {
		ts := getTimeFromID(key.ID)
		if (nowMs - ts) > evConsistentDeltaMs {
			lastIndex = idx
			break
		}
	}
	m.RecentChanges = append(m.RecentChanges[:lastIndex], key)
	sort.Slice(m.RecentChanges, func(i, j int) bool { return m.RecentChanges[i].ID < m.RecentChanges[j].ID })
}

var beginningOfTimeMs = time.Date(2015, time.June, 1, 0, 0, 0, 0, time.UTC).UnixNano() / int64(time.Millisecond)

const sortableIDMask = int64((uint64(1) << 63) - 1)

// Task id is a 64 bits integer represented as a string to the user:
// - 1 highest order bits set to 0 to keep value positive.
// - 43 bits is time since _BEGINING_OF_THE_WORLD at 1ms resolution.
// 	It is good for 2**43 / 365.3 / 24 / 60 / 60 / 1000 = 278 years or 2010+278 =
// 	2288. The author will be dead at that time.
// - 16 bits set to a random value or a server instance specific value. Assuming
// 	an instance is internally consistent with itself, it can ensure to not reuse
// 	the same 16 bits in two consecutive requests and/or throttle itself to one
// 	request per millisecond.
// 	Using random value reduces to 2**-15 the probability of collision on exact
// 	same timestamp at 1ms resolution, so a maximum theoretical rate of 65536000
// 	requests/sec but an effective rate in the range of ~64k requests/sec without
// 	much transaction conflicts. We should be fine.
// - 4 bits set to 0x1. This is to represent the 'version' of the entity schema.
// 	Previous version had 0. Note that this value is XOR'ed in the DB so it's
// 	stored as 0xE. When the TaskRequest entity tree is modified in a breaking
// 	way that affects the packing and unpacking of task ids, this value should be
// 	bumped.
// The key id is this value XORed with task_pack.TASK_REQUEST_KEY_ID_MASK. The
// reason is that increasing key id values are in decreasing timestamp order.
//
// https://github.com/luci/luci-py/blob/master/appengine/swarming/server/task_request.py#L1078

func getSortableTimeID(timeStampMs int64) int64 {
	delta := timeStampMs - beginningOfTimeMs
	random16Bits := rand.Int63() & 0x0FFFF
	id := (delta << 20) | (random16Bits << 4) | 1
	ret := id ^ sortableIDMask
	return ret
}

func getTimeFromID(id int64) int64 {
	return ((id ^ sortableIDMask) >> 20) + beginningOfTimeMs
}
