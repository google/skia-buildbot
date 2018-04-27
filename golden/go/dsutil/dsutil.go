package dsutil

import (
	"math/rand"
	"sort"
	"time"

	"cloud.google.com/go/datastore"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/util"
)

var (
	// The time delta that should guarantee that the data are consistent in ms.
	evConsistentDeltaMs int64 = 3600000
)

type ListHelper struct {
}

func (l *ListHelper) Add(tx *datastore.Transaction, key *datastore.Key) error {
	return nil
}

func (l *ListHelper) GetRecent() (KeySlice, error) {
	return nil, nil
}

type keyContainer struct {
	RecentChanges []*datastore.Key `datastore:",noindex"`
}

func emptyKeyContainer() *keyContainer {
	return &keyContainer{
		RecentChanges: []*datastore.Key{},
	}
}

func (m *keyContainer) update(key *datastore.Key) {
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
	m.RecentChanges = append(m.RecentChanges[:lastIndex], key)
	sort.Slice(m.RecentChanges, func(i, j int) bool { return m.RecentChanges[i].ID < m.RecentChanges[j].ID })
}

type KeySlice []*datastore.Key

func (k KeySlice) Merge(keys []*datastore.Key) []*datastore.Key {
	return nil
}

var beginningOfTimeMs = time.Date(2015, time.June, 1, 0, 0, 0, 0, time.UTC).UnixNano() / int64(time.Millisecond)

const sortableIDMask = int64((uint64(1) << 63) - 1)

func TimedKey(kind ds.Kind) *datastore.Key {
	ret := ds.NewKey(kind)
	ret.ID = getSortableTimeID(util.TimeStamp(time.Millisecond))
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

// package dsutil

// import (
// 	"context"
// 	"fmt"
// 	"sort"
// 	"time"

// 	"go.skia.org/infra/go/eventbus"
// 	"golang.org/x/sync/errgroup"

// 	"cloud.google.com/go/datastore"

// 	"go.skia.org/infra/go/ds"
// 	"go.skia.org/infra/go/sklog"
// 	"go.skia.org/infra/go/util"
// 	"go.skia.org/infra/golden/go/types"
// )

// const (
// 	// EV_TRYJOB_EXP_CHANGED is the event type that is fired when the expectations
// 	// for an issue change. It sends an instance of *TryjobExpChange.
// 	EV_TRYJOB_EXP_CHANGED = "expstorage:tryjob-exp-change"
// )

// type cloudExpStore struct {
// 	issueID  int64
// 	client   *datastore.Client
// 	eventBus eventbus.EventBus

// 	// Use different entities depending on whether this manages the master
// 	// or issue expectations.
// 	changeEntity        ds.Kind
// 	testDigestExpEntity ds.Kind
// 	summaryEntity       ds.Kind
// 	eventExpChange      string
// 	globalEvent         bool
// }

// type IssueExpStoreFactory func(issueID int64) ExpectationsStore

// func NewCloudExpectationsStore(client *datastore.Client, eventBus eventbus.EventBus) (ExpectationsStore, IssueExpStoreFactory, error) {
// 	if client == nil {
// 		return nil, nil, sklog.FmtErrorf("Received nil for datastore client.")
// 	}

// 	store := &cloudExpStore{
// 		issueID:             -1,
// 		changeEntity:        ds.MASTER_EXP_CHANGE,
// 		testDigestExpEntity: ds.MASTER_TEST_DIGEST_EXP,
// 		summaryEntity:       ds.MASTER_EXP_SUMMARY,
// 		eventExpChange:      EV_EXPSTORAGE_CHANGED,
// 		globalEvent:         true,
// 		client:              client,
// 		eventBus:            eventBus,
// 	}

// 	factory := func(issueID int64) ExpectationsStore {
// 		return &cloudExpStore{
// 			issueID:             issueID,
// 			changeEntity:        ds.TRYJOB_EXP_CHANGE,
// 			testDigestExpEntity: ds.TRYJOB_TEST_DIGEST_EXP,
// 			summaryEntity:       ds.ISSUE_EXP_SUMMARY,
// 			eventExpChange:      EV_TRYJOB_EXP_CHANGED,
// 			globalEvent:         false,
// 			client:              client,
// 			eventBus:            eventBus,
// 		}
// 	}

// 	return store, factory, nil
// }

// // Get the current classifications for image digests. The keys of the
// // expectations map are the test names.
// func (c *cloudExpStore) Get() (exp *Expectations, err error) {
// 	currentExp, err := c.getCurrentExpectations(nil)
// 	if err != nil {
// 		return nil, err
// 	}

// 	return currentExp.toExpectations(true), nil
// }

// // AddChange writes the given classified digests to the database and records the
// // user that made the change.
// func (c *cloudExpStore) AddChange(changes map[string]types.TestClassification, userId string) error {
// 	return c.makeChange(changes, userId, 0)
// }

// func (c *cloudExpStore) makeChange(changes map[string]types.TestClassification, userId string, undoChangeID int64) (err error) {
// 	// Write the change record.
// 	ctx := context.Background()
// 	timeStampMs := util.TimeStamp(time.Millisecond)
// 	expChange := &ExpChange{
// 		IssueID:      c.issueID,
// 		UserID:       userId,
// 		UndoChangeID: undoChangeID,
// 		TimeStamp:    timeStampMs,
// 	}

// 	var changeKey *datastore.Key
// 	// if changeKey, err = c.client.Put(ctx, ds.NewKey(c.changeEntity), expChange); err != nil {
// 	if changeKey, err = c.client.Put(ctx, c.timeSortableKey(timeStampMs, c.changeEntity), expChange); err != nil {
// 		return err
// 	}

// 	// If we have an error later make sure to delete change record.
// 	defer func() {
// 		if err != nil {
// 			go func() {
// 				if err := c.deleteExpChanges([]*datastore.Key{changeKey}); err != nil {
// 					sklog.Errorf("Error deleting expectation change %s: %s", changeKey.String(), err)
// 				}
// 			}()
// 		}
// 	}()

// 	// Write the changed expectations to the database.
// 	tcKeys, testChanges := buildExpCollection(changes, c.testDigestExpEntity, changeKey)
// 	if _, err = c.client.PutMulti(ctx, tcKeys, testChanges); err != nil {
// 		return err
// 	}

// 	updateFn := func(tx *datastore.Transaction) error {
// 		// Start transaction to:
// 		//	- add the change to the summary.
// 		//  - store the latest entries to deal eventual consistency.
// 		//  - mark the change as valid.
// 		var egroup errgroup.Group

// 		// Update the recent changes so we get full consistency on queries.
// 		egroup.Go(func() error {
// 			matView, err := c.getRecentExpChanges(tx)
// 			if err != nil {
// 				return err
// 			}
// 			matView.Update(changeKey)
// 			_, err = tx.Put(c.getSummaryKey(), matView)
// 			return err
// 		})

// 		// Update the overall expectations.
// 		egroup.Go(func() error { return c.updateExpectations(tx, changes) })

// 		if err := egroup.Wait(); err != nil {
// 			return err
// 		}

// 		// Mark the expectation change as valid.
// 		expChange.OK = true
// 		if _, err = tx.Put(changeKey, expChange); err != nil {
// 			return err
// 		}
// 		return nil
// 	}

// 	// Run the relevant updates in a transaction.
// 	if _, err = c.client.RunInTransaction(ctx, updateFn); err != nil {
// 		return err
// 	}

// 	if c.eventBus != nil {
// 		c.eventBus.Publish(c.eventExpChange, evExpChange(changes, c.issueID), c.globalEvent)
// 	}
// 	return nil
// }

// func (c *cloudExpStore) updateExpectations(tx *datastore.Transaction, changes map[string]types.TestClassification) error {
// 	currentExp, err := c.getCurrentExpectations(tx)
// 	if err != nil {
// 		return err
// 	}
// 	currentExp.update(changes)
// 	_, err = c.putCurrentExpectations(tx, currentExp)
// 	return err
// }

// func (c *cloudExpStore) getRecentExpChanges(tx *datastore.Transaction) (*MatView, error) {
// 	ret := emptyMatView()
// 	var err error
// 	if tx != nil {
// 		err = tx.Get(c.getSummaryKey(), ret)
// 	} else {
// 		err = c.client.Get(context.Background(), c.getSummaryKey(), ret)
// 	}
// 	if err == nil || err == datastore.ErrNoSuchEntity {
// 		return ret, nil
// 	}
// 	return nil, err
// }

// func (c *cloudExpStore) getCurrentExpectations(tx *datastore.Transaction) (ExpCollection, error) {
// 	_, ret, err := c.getTestDigestExps(tx, c.getSummaryKey(), false)
// 	if err != nil {
// 		return nil, err
// 	}
// 	return ret, nil
// }

// func (c *cloudExpStore) putCurrentExpectations(tx *datastore.Transaction, currentExp ExpCollection) ([]*datastore.PendingKey, error) {
// 	keys, err := currentExp.getKeys(c.testDigestExpEntity, c.getSummaryKey())
// 	if err != nil {
// 		return nil, err
// 	}
// 	return tx.PutMulti(keys, currentExp)
// }

// // QueryLog allows to paginate through the changes in the expectations.
// // If details is true the result will include a list of triage operations
// // that were part a change.
// func (c *cloudExpStore) QueryLog(offset, size int, details bool) ([]*TriageLogEntry, int, error) {
// 	allKeys, err := c.getExpChangeKeys(0)
// 	if err != nil {
// 		return nil, 0, sklog.FmtErrorf("Error retrieving keys for expectation changes: %s", err)
// 	}

// 	if offset < 0 {
// 		offset = 0
// 	}

// 	if size <= 0 {
// 		size = len(allKeys)
// 	}

// 	start := util.MinInt(offset, len(allKeys))
// 	end := util.MinInt(start+size, len(allKeys))
// 	retKeys := allKeys[start:end]

// 	ret := make([]*TriageLogEntry, 0, len(retKeys))
// 	var detailRecs [][]*TriageDetail = nil
// 	var egroup errgroup.Group
// 	egroup.Go(func() error {
// 		expChanges := make([]*ExpChange, len(retKeys))
// 		if err := c.client.GetMulti(context.Background(), retKeys, expChanges); err != nil {
// 			return sklog.FmtErrorf("Error retrieving expectation changes: %s", err)
// 		}

// 		for _, change := range expChanges {
// 			ret = append(ret, &TriageLogEntry{
// 				ID:           change.ChangeID.ID,
// 				Name:         change.UserID,
// 				TS:           change.TimeStamp,
// 				ChangeCount:  int(change.Count),
// 				Details:      nil,
// 				UndoChangeID: change.UndoChangeID,
// 			})
// 		}
// 		return nil
// 	})

// 	// If we want details fetch them in parallel to the change records.
// 	if details {
// 		detailRecs = make([][]*TriageDetail, len(retKeys))
// 		for idx, parentKey := range retKeys {
// 			func(idx int, parentKey *datastore.Key) {
// 				egroup.Go(func() error {
// 					_, expColl, err := c.getTestDigestExps(nil, parentKey, false)
// 					if err != nil {
// 						return sklog.FmtErrorf("Error retrieving change details: %s", err)
// 					}

// 					triageDetails := make([]*TriageDetail, 0, len(expColl[0].Names))
// 					for _, batch := range expColl {
// 						for nameIdx, name := range batch.Names {
// 							triageDetails = append(triageDetails, &TriageDetail{
// 								TestName: name,
// 								Digest:   batch.Digests[nameIdx],
// 								Label:    batch.Labels[nameIdx],
// 							})
// 						}
// 					}

// 					sort.Slice(triageDetails, func(i, j int) bool {
// 						return ((triageDetails[i].TestName < triageDetails[j].TestName) ||
// 							(triageDetails[i].Digest < triageDetails[j].Digest))
// 					})

// 					detailRecs[idx] = triageDetails
// 					return nil
// 				})
// 			}(idx, parentKey)
// 		}
// 	}

// 	// Wait for all queries to finish.
// 	if err := egroup.Wait(); err != nil {
// 		return nil, 0, err
// 	}

// 	// Fill in the details.
// 	if details {
// 		for idx, triageEntry := range ret {
// 			triageEntry.Details = detailRecs[idx]
// 		}
// 	}

// 	return ret, len(allKeys), nil
// }

// // UndoChange reverts a change by setting all testname/digest pairs of the
// // original change to the label they had before the change was applied.
// // A new entry is added to the log with a reference to the change that was
// // undone.
// func (c *cloudExpStore) UndoChange(changeID int64, userID string) (map[string]types.TestClassification, error) {
// 	// Make sure the entity is valid.
// 	if changeID <= 0 {
// 		return nil, sklog.FmtErrorf("Change with id %d does not exist.", changeID)
// 	}

// 	// Fetch the change and it's details.
// 	expChange := &ExpChange{}
// 	expChangeKey := ds.NewKey(c.changeEntity)
// 	expChangeKey.ID = changeID
// 	var egroup errgroup.Group
// 	egroup.Go(func() error {
// 		if err := c.client.Get(context.Background(), expChangeKey, expChange); err != nil {
// 			if err == datastore.ErrNoSuchEntity {
// 				return sklog.FmtErrorf("Change with id %d does not exist.", changeID)
// 			}
// 			return err
// 		}
// 		return nil
// 	})

// 	var changes map[string]types.TestClassification = nil
// 	egroup.Go(func() error {
// 		var err error
// 		var coll ExpCollection
// 		if _, coll, err = c.getTestDigestExps(nil, expChangeKey, false); err != nil {
// 			return sklog.FmtErrorf("Error retrieving expectation changes for change %d", changeID)
// 		}
// 		changes = coll.toExpectations(false).Tests
// 		for _, digests := range changes {
// 			for d := range digests {
// 				digests[d] = types.UNTRIAGED
// 			}
// 		}
// 		return nil
// 	})

// 	if err := egroup.Wait(); err != nil {
// 		return nil, err
// 	}

// 	// If this has been undone already, then don't do it.
// 	if expChange.UndoChangeID != 0 {
// 		return nil, fmt.Errorf("Unable to undo change %d which was created as an undo of change %d.", changeID, expChange.UndoChangeID)
// 	}

// 	allKeys, err := c.getExpChangeKeys(changeID)
// 	if err != nil {
// 		return nil, sklog.FmtErrorf("Error retrieving keys for expectation changes: %s", err)
// 	}

// 	details := make([]ExpCollection, len(allKeys))
// 	for idx, parentKey := range allKeys {
// 		func(idx int, parentKey *datastore.Key) {
// 			egroup.Go(func() error {
// 				var err error
// 				_, details[idx], err = c.getTestDigestExps(nil, parentKey, false)
// 				return err
// 			})
// 		}(idx, parentKey)
// 	}

// 	if err := egroup.Wait(); err != nil {
// 		return nil, sklog.FmtErrorf("Error retrieving change details: %s", err)
// 	}

// 	// Build the change that we need to make for the undo.
// 	for idx := len(details) - 1; idx >= 0; idx-- {
// 		for _, batch := range details[idx] {
// 			for idx, name := range batch.Names {
// 				digest := batch.Digests[idx]
// 				if _, ok := changes[name][digest]; ok {
// 					changes[name][digest] = types.LabelFromString(batch.Labels[idx])
// 				}
// 			}
// 		}
// 	}
// 	return changes, c.makeChange(changes, userID, changeID)
// }

// // TODO(stephana): The removeChange function is obsolete and should be removed.
// // It was used for testing before the UndoChange function was added. It is simply
// // wrong to change the expectations

// // removeChange removes the given digests from the expectations store.
// // The key in changes is the test name which maps to a list of digests
// // to remove. Used for testing only.
// func (c *cloudExpStore) removeChange(changes map[string]types.TestClassification) error {
// 	for _, digests := range changes {
// 		for digest := range digests {
// 			digests[digest] = types.UNTRIAGED
// 		}
// 	}

// 	updateFn := func(tx *datastore.Transaction) error {
// 		return c.updateExpectations(tx, changes)
// 	}

// 	// Run the removal changes in a transaction.
// 	if _, err := c.client.RunInTransaction(context.Background(), updateFn); err != nil {
// 		return err
// 	}

// 	if c.eventBus != nil {
// 		// This is always a local event since it's only used for testing.
// 		c.eventBus.Publish(c.eventExpChange, evExpChange(changes, c.issueID), false)
// 	}
// 	return nil
// }

// // getExpChangeKeys returns all the expectation changes for the given issue
// // in revers chronological order. offset and size pick a subset of the result.
// // Both are only considered if they are larger than 0. keysOnly indicates that we
// // want keys only.
// func (c *cloudExpStore) getExpChangeKeys(atBeforeID int64) ([]*datastore.Key, error) {
// 	// Qquery all changes
// 	var egroup errgroup.Group
// 	var queryKeys []*datastore.Key
// 	egroup.Go(func() error {
// 		q := ds.NewQuery(c.changeEntity).
// 			Filter("OK =", true).
// 			KeysOnly()

// 		if c.issueID > 0 {
// 			q = q.Filter("IssueID =", c.issueID)
// 		}

// 		var err error
// 		queryKeys, err = c.client.GetAll(context.Background(), q, nil)
// 		return err
// 	})

// 	// Load the recent added changes.
// 	var matView *MatView
// 	egroup.Go(func() error {
// 		var err error
// 		matView, err = c.getRecentExpChanges(nil)
// 		return err
// 	})

// 	if err := egroup.Wait(); err != nil {
// 		return nil, err
// 	}

// 	// Concatenate the result, sort it and filter out duplicates.
// 	allKeys := append(matView.RecentChanges, queryKeys...)
// 	sort.Slice(allKeys, func(i, j int) bool { return allKeys[i].ID < allKeys[j].ID })
// 	ret := make([]*datastore.Key, 0, len(allKeys))
// 	lastID := int64(-1)
// 	for _, key := range allKeys {
// 		// Filter out the target key if one was given and also remove duplicates.
// 		if (key.ID != lastID) && (key.ID != atBeforeID) {
// 			ret = append(ret, key)
// 			lastID = key.ID
// 		}
// 	}

// 	// Remove all keys that are newer than the target key
// 	if atBeforeID > 0 {
// 		ts := getTimeFromID(atBeforeID)
// 		for idx, key := range ret {
// 			if getTimeFromID(key.ID) <= ts {
// 				ret = ret[idx:]
// 				break
// 			}
// 		}
// 	}

// 	return ret, nil
// }

// // getTestDigstExpectations gets all expectations for the given change.
// func (c *cloudExpStore) getTestDigestExps(tx *datastore.Transaction, parentKey *datastore.Key, keysOnly bool) ([]*datastore.Key, ExpCollection, error) {
// 	q := ds.NewQuery(c.testDigestExpEntity).Ancestor(parentKey)

// 	if tx != nil {
// 		q = q.Transaction(tx)
// 	}

// 	if keysOnly {
// 		q = q.KeysOnly()
// 	}

// 	var exps ExpCollection
// 	expsKeys, err := c.client.GetAll(context.Background(), q, &exps)
// 	if err != nil {
// 		return nil, nil, err
// 	}
// 	return expsKeys, exps, nil
// }

// // deleteExpChanges deletes the given expectation changes.
// func (c *cloudExpStore) deleteExpChanges(keys []*datastore.Key) error {
// 	return c.client.DeleteMulti(context.Background(), keys)
// }

// // getIssueKey returns a datastore key for the given issue id.
// func (c *cloudExpStore) getSummaryKey() *datastore.Key {
// 	ret := ds.NewKey(c.summaryEntity)
// 	ret.ID = c.issueID
// 	return ret
// }

// func (c *cloudExpStore) timeSortableKey(timeStampMs int64, kind ds.Kind) *datastore.Key {
// 	ret := ds.NewKey(kind)
// 	ret.ID = getSortableTimeID(timeStampMs)
// 	return ret
// }
