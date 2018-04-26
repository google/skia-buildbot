package dsutil

import (
	"context"
	"math/rand"
	"sort"
	"time"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/util"
)

const (
	// DefaultConsistencyDelta is the default (assumed) time it takes for query results to be consistent
	DefaultConsistencyDelta time.Duration = 30 * 60 * 1000 * time.Millisecond
)

type KeySlice []*datastore.Key

func (k KeySlice) Merge(keys []*datastore.Key) []*datastore.Key {
	// Make sure the slice it sorted at the end.
	var ret []*datastore.Key
	ret = make([]*datastore.Key, 0, len(k)+len(keys))
	ret = append(ret, k...)
	ret = append(ret, keys...)
	sort.Slice(ret, func(i, j int) bool { return ret[i].ID < ret[j].ID })

	// Filter out duplicates.
	lastIdx := 0
	lastID := int64(-1)
	for _, key := range ret {
		if key.ID != lastID {
			ret[lastIdx] = key
			lastIdx++
			lastID = key.ID
		}
	}
	return ret[:lastIdx]
}

type ListHelper struct {
	client             *datastore.Client
	containerKey       *datastore.Key
	consistencyDeltaMs int64
}

func NewListHelper(client *datastore.Client, kind ds.Kind, entityName string, consistentDelta time.Duration) *ListHelper {
	containerKey := ds.NewKey(kind)
	containerKey.Name = entityName

	return &ListHelper{
		client:             client,
		containerKey:       containerKey,
		consistencyDeltaMs: int64(consistentDelta / time.Millisecond),
	}
}

func (l *ListHelper) Add(tx *datastore.Transaction, newKey *datastore.Key) error {
	return l.updateRecentKeys(tx, newKey, false)
}

func (l *ListHelper) Delete(tx *datastore.Transaction, removeKey *datastore.Key) error {
	return l.updateRecentKeys(tx, removeKey, true)
}

func (l *ListHelper) updateRecentKeys(tx *datastore.Transaction, key *datastore.Key, remove bool) error {
	recent := emptyKeyContainer()
	if err := tx.Get(l.containerKey, recent); err != nil && err != datastore.ErrNoSuchEntity {
		return err
	}

	// Update the current keys.
	recent.update(key, l.consistencyDeltaMs, remove)
	_, err := tx.Put(l.containerKey, recent)
	return err
}

func (l *ListHelper) GetRecent() (KeySlice, error) {
	ret := &keyContainer{}
	if err := l.client.Get(context.Background(), l.containerKey, ret); err != nil {
		return nil, err
	}

	return KeySlice(ret.RecentChanges), nil
}

// Helper type.
type keyContainer struct {
	RecentChanges []*datastore.Key `datastore:",noindex"`
}

func emptyKeyContainer() *keyContainer {
	return &keyContainer{
		RecentChanges: []*datastore.Key{},
	}
}

func (m *keyContainer) update(keyToUpdate *datastore.Key, evConsistentDeltaMs int64, remove bool) {
	if remove {
		// Search for the key and remove it if we find it.
		idx := sort.Search(len(m.RecentChanges), func(i int) bool {
			return m.RecentChanges[i].ID >= keyToUpdate.ID
		})
		if idx != len(m.RecentChanges) && m.RecentChanges[idx].ID == keyToUpdate.ID {
			m.RecentChanges = append(m.RecentChanges[:idx], m.RecentChanges[idx+1:]...)
		}
	}

	// Filter out the existing
	nowMs := util.TimeStamp(time.Millisecond)
	lastIndex := len(m.RecentChanges)
	for idx, key := range m.RecentChanges {
		ts := GetTimeFromID(key.ID)
		if (nowMs - ts) > evConsistentDeltaMs {
			lastIndex = idx
			break
		}
	}

	// We have removed it at this point and it's sorted by default. We are done.
	if remove {
		return
	}

	// Add the new key and sort the list.
	m.RecentChanges = append(m.RecentChanges[:lastIndex], keyToUpdate)
	sort.Slice(m.RecentChanges, func(i, j int) bool { return m.RecentChanges[i].ID < m.RecentChanges[j].ID })
}

var beginningOfTimeMs = time.Date(2015, time.June, 1, 0, 0, 0, 0, time.UTC).UnixNano() / int64(time.Millisecond)

const sortableIDMask = int64((uint64(1) << 63) - 1)

func TimeSortableKey(kind ds.Kind, timeStampMs int64) *datastore.Key {
	ret := ds.NewKey(kind)
	if timeStampMs == 0 {
		timeStampMs = util.TimeStamp(time.Millisecond)
	}
	ret.ID = getSortableTimeID(timeStampMs)
	return ret
}

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

func GetTimeFromID(id int64) int64 {
	return ((id ^ sortableIDMask) >> 20) + beginningOfTimeMs
}
