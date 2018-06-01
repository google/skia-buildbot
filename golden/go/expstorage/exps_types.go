package expstorage

import (
	"bytes"
	"context"
	"encoding/json"
	"math"

	"cloud.google.com/go/datastore"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/types"
)

const (
	// masterIssueID is the value used for IssueID when we dealing with the
	// master branch. Any IssueID < 0 should be ignored.
	masterIssueID = -1
)

// ExpChange is used to store an expectation change in the database. Each
// expectation change is an atomic change to expectations for an issue.
// The actual expectations are captured in instances of TestDigestExp.
type ExpChange struct {
	ChangeID     *datastore.Key `datastore:"__key__"`
	IssueID      int64
	UserID       string
	TimeStamp    int64 `datastore:",noindex"`
	Count        int64 `datastore:",noindex"`
	UndoChangeID int64
	OK           bool
	Children     []*datastore.Key `datastore:",noindex"`
}

// // nDigestsPerRec is the number of (Testname, Digest, Label) triples we store
// // in a single instance of TestDigestExp. The value is chosen to reliably fit
// // into the space limits of a datastore instance.
// //
// // A single entity can contain about 1 MiB
// // (See https://cloud.google.com/datastore/docs/concepts/limits).
// //
// // Each digest is 32 characters and the label is 8 bytes (stored as an integer)
// // Since the test names are limited to 256 bytes (see types.MAXIMUM_NAME_LENGTH)
// // and we need one byte to terminate strings, we get
// //    nDigestsPerRec = 1000000/(256 + 32 + 8 + 2) ~ 3355
// // We are rounding down throughout this calculation to make a conservative
// // estimate.
// const nDigestsPerRec = 3000

// // TestDigestExp is used to store expectations for an issue in the database.
// // It stores nDigestsPerRec expectations in each entity so we can retrieve
// // many expectations at once.
// type TestDigestExp struct {
// 	Key     *datastore.Key `datastore:"__key__"` // Key is populated when the entity is loaded.
// 	Names   []string       `datastore:",noindex"`
// 	Digests []string       `datastore:",noindex"`
// 	Labels  []types.Label  `datastore:",noindex"`
// }

// // newTestDigestExp allocates a new block for hold expectations.
// func newTestDigestExp() *TestDigestExp {
// 	return &TestDigestExp{
// 		Names:   make([]string, 0, nDigestsPerRec),
// 		Digests: make([]string, 0, nDigestsPerRec),
// 		Labels:  make([]types.Label, 0, nDigestsPerRec),
// 	}
// }

// // full returns true if this batch of expectations is full and a new one should
// // be allocated
// func (e *TestDigestExp) full() bool {
// 	return len(e.Names) >= nDigestsPerRec
// }

// // add adds an new triple to the expectations. It does not check whether the
// // current block is full.
// func (e *TestDigestExp) add(name, digest string, label types.Label) {
// 	e.Names = append(e.Names, name)
// 	e.Digests = append(e.Digests, digest)
// 	e.Labels = append(e.Labels, label)
// }

// // TDESlice is a slice of TestDigestExp allowing to store an arbitrary number of
// // expectations in multiple blocks.
// type TDESlice []*TestDigestExp

// func (e TDESlice) empty() bool {
// 	return (len(e) == 0) || (len(e[0].Names) == 0)
// }

// // buildTDESlice converts the given expectation(change)s to a TDESlice instance
// // for storage in the cloud datastore.
// func buildTDESlice(expChange map[string]types.TestClassification) TDESlice {
// 	expCol := TDESlice{newTestDigestExp()}

// 	// Assemble the collection of expectations.
// 	for testName, classification := range expChange {
// 		for digest, label := range classification {
// 			expCol.add(testName, digest, label)
// 		}
// 	}

// 	return expCol
// }

// // add adds a new expectation to the current TDESlice
// func (e *TDESlice) add(name, digest string, label types.Label) {
// 	curr := (*e)[len(*e)-1]
// 	if curr.full() {
// 		curr = newTestDigestExp()
// 		*e = append(*e, curr)
// 	}
// 	curr.add(name, digest, label)
// }

// // update the existing collection of expectations.
// func (e *TDESlice) update(triagedDigests map[string]types.TestClassification) {
// 	// If the collection is empty then just build a new one.
// 	if len(*e) == 0 {
// 		*e = buildTDESlice(triagedDigests)
// 		return
// 	}

// 	// Make a copy of the changes to keep track of the ones we have already accounted for.
// 	change := (&Expectations{Tests: triagedDigests}).DeepCopy().Tests

// 	// empty keeps track of spots that have been changed to untriaged and can
// 	// be overridden. This avoids fragmentation of the batches of expectations.
// 	empty := []int{}

// 	for batchIdx, exp := range *e {
// 		for idx, name := range exp.Names {
// 			digest := exp.Digests[idx]
// 			if newLabel, ok := change[name][digest]; ok {
// 				// Update the label and remove the entry.
// 				exp.Labels[idx] = newLabel
// 				delete(change[name], digest)
// 			}
// 			// Mark untriaged as empty slots for new entries to avoid fragmentation.
// 			if exp.Labels[idx] == types.UNTRIAGED {
// 				empty = append(empty, batchIdx, idx)
// 			}
// 		}
// 	}

// 	emptyIdx := 0
// 	for name, digests := range change {
// 		for digest, label := range digests {
// 			// If we still have empty slots then insert this expectation.
// 			if emptyIdx < len(empty) {
// 				batch := (*e)[empty[emptyIdx]]
// 				idx := empty[emptyIdx+1]
// 				emptyIdx += 2
// 				batch.Names[idx] = name
// 				batch.Digests[idx] = digest
// 				batch.Labels[idx] = label
// 			} else {
// 				e.add(name, digest, label)
// 			}
// 		}
// 	}
// }

// // convert the expectations to the datastructure that is easier for lookup.
// func (e TDESlice) toExpectations(filterUntriaged bool) *Expectations {
// 	ret := map[string]types.TestClassification{}
// 	for _, exp := range e {
// 		for idx, name := range exp.Names {
// 			digest := exp.Digests[idx]
// 			label := exp.Labels[idx]
// 			if filterUntriaged && (label == types.UNTRIAGED) {
// 				continue
// 			}

// 			testEntry, ok := ret[name]
// 			if !ok {
// 				ret[name] = types.TestClassification{digest: label}
// 			} else {
// 				testEntry[digest] = label
// 			}
// 		}
// 	}
// 	return &Expectations{
// 		Tests: ret,
// 	}
// }

// // getKeys returns the keys for this collection of expectations. If an instance
// // of TestDigestExp does not contain a key (because it was not loaded from the
// // datastore) we create a new key.
// func (e TDESlice) getKeys(kind ds.Kind, parentKey *datastore.Key) []*datastore.Key {
// 	ret := make([]*datastore.Key, len(e))
// 	for idx, entry := range e {
// 		ret[idx] = entry.Key
// 		if ret[idx] == nil {
// 			ret[idx] = ds.NewKeyWithParent(kind, parentKey)
// 		}
// 	}
// 	return ret
// }

// EventExpectationChange is the structure that is sent in expectation change events.
// When the change happened on the master branch 'IssueID' will contain a value <0
// and should be ignored.
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

type BlobParent struct {
	Children []*datastore.Key
	keysIdx  int
	props    []datastore.Property
}

func keysFromISlice(iSlice []interface{}) []*datastore.Key {
	ret := make([]*datastore.Key, len(iSlice))
	for idx, k := range iSlice {
		ret[idx] = k.(*datastore.Key)
	}
	return ret
}

func iSliceFromKeys(keys []*datastore.Key) []interface{} {
	ret := make([]interface{}, len(keys))
	for idx, k := range keys {
		ret[idx] = k
	}
	return ret
}

func (b *BlobParent) Load(ps []datastore.Property) error {
	b.props = ps
	for idx := range ps {
		if ps[idx].Name == "Children" {
			b.Children = keysFromISlice(ps[idx].Value.([]interface{}))
			break
		}
	}
	return nil
}

func (b *BlobParent) Save() ([]datastore.Property, error) {
	found := false
	iSlice := iSliceFromKeys(b.Children)
	for idx := range b.props {
		if b.props[idx].Name == "Children" {
			b.props[idx].Value = iSlice
			found = true
			break
		}
	}

	if !found {
		b.props = append(b.props, datastore.Property{Name: "Children", Value: iSlice, NoIndex: true})
	}
	return b.props, nil
}

type BlobPart struct {
	ID   int32  `datastore:",noindex"`
	Body []byte `datastore:",noindex"`
}

// buildTDESlice converts the given expectation(change)s to a TDESlice instance
// for storage in the cloud datastore.
func jsonEncodeBlobParts(src interface{}) ([]*BlobPart, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(src); err != nil {
		return nil, err
	}
	body := buf.Bytes()
	sklog.Infof("Buf size: %d", len(body))

	pageSize := 1000*1000 - 1000
	nPages := int(math.Ceil(float64(len(body)) / float64(pageSize)))
	sklog.Infof("Buf size: %d, pages: %d", len(body), nPages)

	ret := make([]*BlobPart, nPages)
	for p := 0; p < nPages; p++ {
		start := p * pageSize
		ret[p] = &BlobPart{
			ID:   int32(p),
			Body: body[start:util.MinInt(start+pageSize, len(body))],
		}
	}
	return ret, nil
}

func jsonDecodeBlobParts(parts []*BlobPart, dst interface{}) error {
	var buf bytes.Buffer
	for _, part := range parts {
		if _, err := buf.Write(part.Body); err != nil {
			return err
		}
	}
	return json.NewDecoder(&buf).Decode(dst)
}

type BlobLoader struct {
	client *datastore.Client
}

func (b *BlobLoader) LoadJsonBlob(tx *datastore.Transaction, parentKey *datastore.Key, dst interface{}) (*BlobParent, error) {
	getFn := GetFn(b.client, tx)
	parent := &BlobParent{}
	if err := getFn(parentKey, parent); err != nil {
		if err == datastore.ErrNoSuchEntity {
			return nil, nil
		}
		return nil, sklog.FmtErrorf("Error loading the blob parent: %s", err)
	}

	// TODO(stephana): If we we are not in a transaction the parts of the blob could
	// be read in parallel.
	blobParts := make([]*BlobPart, len(parent.Children))
	for idx, key := range parent.Children {
		blobParts[idx] = &BlobPart{}
		if err := getFn(key, blobParts[idx]); err != nil {
			return nil, sklog.FmtErrorf("Error loading blob parts #v", err)
		}
	}

	if err := jsonDecodeBlobParts(blobParts, dst); err != nil {
		return nil, sklog.FmtErrorf("Error decoding JSON from blobs: %s", err)
	}
	return parent, nil
}

func (b *BlobLoader) UpdateJsonBlob(tx *datastore.Transaction, parentKey *datastore.Key, parent *BlobParent, kind ds.Kind, value interface{}) (*datastore.Key, error) {
	defer timer.New("updateJsonBlob").Stop()
	if parent != nil {
		if err := b.DeleteBlob(tx, parentKey, parent); err != nil {
			return nil, err
		}
	} else {
		parent = &BlobParent{}
	}

	var err error
	parent.Children, err = b.WriteJsonBlobParts(kind, value)
	if err != nil {
		return nil, err
	}

	putFn := PutFn(b.client, tx)
	return putFn(parentKey, parent)
}

func (b *BlobLoader) WriteJsonBlobParts(kind ds.Kind, value interface{}) ([]*datastore.Key, error) {
	blobParts, err := jsonEncodeBlobParts(value)
	if err != nil {
		return nil, err
	}

	keys := make([]*datastore.Key, len(blobParts))
	var egroup errgroup.Group
	ctx := context.TODO()
	for idx, part := range blobParts {
		func(idx int, part *BlobPart) {
			egroup.Go(func() error {
				var err error
				keys[idx], err = b.client.Put(ctx, ds.NewKey(kind), part)
				return err
			})
		}(idx, part)
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}
	return keys, nil
}

func (b *BlobLoader) DeleteBlob(tx *datastore.Transaction, parentKey *datastore.Key, parent *BlobParent) error {
	if len(parent.Children) == 0 {
		return nil
	}

	deleteMultiFn := DeleteMultiFn(b.client, tx)
	if err := deleteMultiFn(parent.Children); err != nil {
		return err
	}

	parent.Children = []*datastore.Key{}
	putFn := PutFn(b.client, tx)
	_, err := putFn(parentKey, parent)
	return err
}

func GetFn(client *datastore.Client, tx *datastore.Transaction) func(*datastore.Key, interface{}) error {
	if tx != nil {
		return tx.Get
	}
	return func(k *datastore.Key, dst interface{}) error {
		return client.Get(context.TODO(), k, dst)
	}
}

func DeleteMultiFn(client *datastore.Client, tx *datastore.Transaction) func([]*datastore.Key) error {
	if tx != nil {
		return tx.DeleteMulti
	}
	return func(k []*datastore.Key) error {
		return client.DeleteMulti(context.TODO(), k)
	}
}

func PutFn(client *datastore.Client, tx *datastore.Transaction) func(*datastore.Key, interface{}) (*datastore.Key, error) {
	if tx != nil {
		return func(k *datastore.Key, val interface{}) (*datastore.Key, error) {
			_, err := tx.Put(k, val)
			return nil, err
		}
	}
	return func(k *datastore.Key, val interface{}) (*datastore.Key, error) {
		return client.Put(context.TODO(), k, val)
	}
}
