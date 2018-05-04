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

// KeySlics is a wrapper type that adds functions to a slice of datastore keys.
type KeySlice []*datastore.Key

// Merge assumes that all keys are time based (created via TimeSortableKey) and
// sortable. It merges the keys of this slice with the given slice,
// deduplicates them and returns the deduplicated keys sorted in ascendring order
// which means the underlying times are in descending order (newest first).
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

// ListHelper stores recently added keys in an entity for a defined duration.
// This allows to augment an eventually consistent query with the most recent
// keys and therefore yield a consistent listing of entries.
// It assumes that the used keys are time based (created via TimeSortableKey)
// and can easily be sorted and the underlying time can be extracted.
type ListHelper struct {
	// client is the cloud datastore client.
	client *datastore.Client

	// containerKey is the key of the entity where the most recently added keys
	// should be stored.
	containerKey *datastore.Key

	// consistencyDeltaMs is th
	consistencyDeltaMs int64
}

// NewListHelper creates a new instance of ListHelper. It will store any keys
// added via the Add function to the entity identified by the containerKey.
// Any keys in the container entity that are older than the duration given as
// consistentDelta will be removed.
func NewListHelper(client *datastore.Client, containerKey *datastore.Key, consistentDelta time.Duration) *ListHelper {
	return &ListHelper{
		client:             client,
		containerKey:       containerKey,
		consistencyDeltaMs: int64(consistentDelta / time.Millisecond),
	}
}

// Add adds a new key to the set of recently added keys within the given transaction.
// This should be called in the transaction that is used to add the new entity.
// It will add the key to the set of recently added keys and removes any keys
// that are no longer with the defined time delta.
func (l *ListHelper) Add(tx *datastore.Transaction, newKey *datastore.Key) error {
	return l.updateRecentKeys(tx, newKey, false)
}

// Delete removes the given Key from the container that contains the recent keys.
// This needs to be called whenever an entity of the target collection is removed
// to make sure there are no dangling keys in the container.
func (l *ListHelper) Delete(tx *datastore.Transaction, removeKey *datastore.Key) error {
	return l.updateRecentKeys(tx, removeKey, true)
}

// updateRecentKeys adds or removes the given key from the entity that contains
// the list of recently added keys.
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

// GetRecent returns the list of keys of a collection that were recently added.
// This should be called in parallel to an eventually consistent query.
// The returned KeySlice instance can then be used to merge recent keys with the
// result of the query resulting in a consistent snapshot of the collection.
func (l *ListHelper) GetRecent() (KeySlice, error) {
	ret := emptyKeyContainer()
	if err := l.client.Get(context.Background(), l.containerKey, ret); err != nil && err != datastore.ErrNoSuchEntity {
		return nil, err
	}

	return KeySlice(ret.RecentChanges), nil
}

// keyContainer is a helper type that manages a list of sorted keys.
type keyContainer struct {
	RecentChanges []*datastore.Key `datastore:",noindex"`
}

// emptyKeyContainer returns an empty instance of keyContainer.
func emptyKeyContainer() *keyContainer {
	return &keyContainer{
		RecentChanges: []*datastore.Key{},
	}
}

// update adds or removes a key from the list of keys and guarantees that all entries are unique
// and sorted in ascending order (newest first).
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

// beginningOfTimeMs is the reference time used to generate time based unique
// ids in the getSortableTimeID function.
var beginningOfTimeMs = time.Date(2018, time.April, 1, 0, 0, 0, 0, time.UTC).UnixNano() / int64(time.Millisecond)

// sortableIDMask is a mast witht he lowest 63 bits set to 1. Used to invert the id in
// getSortableTimeID.
const sortableIDMask = int64((uint64(1) << 63) - 1)

// TimeSortableKey returns a datastore key for the given kind and timestamp (in ms).
// The returned key has the property that it contains the given timestamp empbedded
// in its numeric ID and that is is sortable. The ID is inverted in a way that
// when sorted in ascending order the embedded timestamps are sorted in decending
// order. Thus the newest keys are first.
// The GetTimeFromID allows to extract the timestamp from the id of the returned key.
func TimeSortableKey(kind ds.Kind, timeStampMs int64) *datastore.Key {
	ret := ds.NewKey(kind)
	if timeStampMs == 0 {
		timeStampMs = util.TimeStamp(time.Millisecond)
	}
	ret.ID = getSortableTimeID(timeStampMs)
	return ret
}

// GetTimeFromID returns a time stamp in ms from the given id. It is a assumed
// that the id comes from a key that was generated with the TimeSortableKey function.
func GetTimeFromID(id int64) int64 {
	return ((id ^ sortableIDMask) >> 20) + beginningOfTimeMs
}

// getSortableTimeID returns a 64-bit ID that contains the current time and
// is inverted and has the property that when sorted in ascending order contains
// time stamps in decreasing order. Thus the newest IDs are first in the ordering.
// This was adapted from (description below copied from there):
//
// https://github.com/luci/luci-py/blob/master/appengine/swarming/server/task_request.py#L1078
//
// The key contains a 64-bit numeric ID that follows this structure:
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
func getSortableTimeID(timeStampMs int64) int64 {
	delta := timeStampMs - beginningOfTimeMs
	random16Bits := rand.Int63() & 0x0FFFF
	id := (delta << 20) | (random16Bits << 4) | 1
	ret := id ^ sortableIDMask
	return ret
}
