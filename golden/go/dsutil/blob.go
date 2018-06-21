package dsutil

import (
	"bytes"
	"context"
	"encoding/json"
	"math"

	"cloud.google.com/go/datastore"
	"go.skia.org/infra/go/ds"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/sync/errgroup"
)

// maxBlobFragBytes is the size of a blob fragment and slightly smaller than a
// the maximum size of an entity which is 1MiB - 4
const maxBlobFragBytes = 1000 * 1000

// BlobStore is a utility type to store data encoded as JSON in blobs. This is
// intended for blobs that are larger than 1MiB. It breaks the blob into multiple
// entities that can be addressed via a single key.
type BlobStore struct {
	client       *datastore.Client
	blobKind     ds.Kind
	blobFragKind ds.Kind
}

// NewBlobStore creates a new instance of BlobStore.
// 'blobKind' is the name of the entity used to store the blob index.
// 'blobFragKind' is the name fo the entity used to store the JSON encoded fragments
//	of the blob
func NewBlobStore(client *datastore.Client, blobKind ds.Kind, blobFragKind ds.Kind) *BlobStore {
	return &BlobStore{
		client:       client,
		blobKind:     blobKind,
		blobFragKind: blobFragKind,
	}
}

// blobParent serves as the index of the blob, it holds the keys of the blob
// fragments that make up the blob. The fragments are intended to be
// dis/assembled in the order of 'Children'
type blobParent struct {
	Children []*datastore.Key `datastore:",noindex"`
}

// blobFrag contains one fragment of the JSON encoded blob data.
type blobFrag struct {
	Body []byte `datastore:",noindex"`
}

// Load loads the blob data referred to by the given key into 'dst'. The same
// rules apply to 'dst' as for json.Decoder.Decode(...)
func (b *BlobStore) Load(key *datastore.Key, dst interface{}) error {
	ctx := context.TODO()
	blob := &blobParent{}
	if err := b.client.Get(ctx, key, blob); err != nil {
		return err
	}
	return b.readBlobData(blob.Children, dst)
}

// Save stores the given data in a blob, which can then be referenced by the
// returned key. Internally the data are json encoded, so the same rules apply
// to 'data' as for json.Encoder.Encode(...)
func (b *BlobStore) Save(data interface{}) (key *datastore.Key, err error) {
	ctx := context.TODO()
	blob := &blobParent{}
	blob.Children, err = b.writeBlobData(data)
	if err != nil {
		return nil, err
	}

	// If we fail we need to purge all the entities we create along the way
	actions := &TxActions{}
	actions.AddRollbackFn(func() error { return b.client.DeleteMulti(ctx, blob.Children) })
	defer func() { actions.Run(err) }()

	key = ds.NewKey(b.blobKind)
	return b.client.Put(ctx, key, blob)
}

// Delete deletes the blob identified by key.
func (b *BlobStore) Delete(key *datastore.Key) error {
	var egroup errgroup.Group
	ctx := context.TODO()

	blob := &blobParent{}
	if err := b.client.Get(ctx, key, blob); err != nil {
		return err
	}

	// Delete the parent and all the children.
	egroup.Go(func() error { return b.client.Delete(ctx, key) })
	egroup.Go(func() error { return b.client.DeleteMulti(ctx, blob.Children) })
	return egroup.Wait()
}

// writeBlobData encodes the given object as JSON and stores it in a
// sequence of entities. It returns the keys of the newly created entities that
// contain the blob.
func (b *BlobStore) writeBlobData(value interface{}) ([]*datastore.Key, error) {
	blobParts, err := jsonEncodeBlobParts(value)
	if err != nil {
		return nil, err
	}

	keys := make([]*datastore.Key, len(blobParts))
	var egroup errgroup.Group
	ctx := context.TODO()
	for idx, part := range blobParts {
		func(idx int, part *blobFrag) {
			egroup.Go(func() error {
				var err error
				keys[idx], err = b.client.Put(ctx, ds.NewKey(b.blobFragKind), part)
				return err
			})
		}(idx, part)
	}

	if err := egroup.Wait(); err != nil {
		return nil, err
	}
	return keys, nil
}

// readBlobData reads the blob parts identified by the given list of keys and
// concatenate their Body fields. The result is then JSON decoded into 'dst'.
// The same rules for dst apply as for json.Decoder.Decode(...).
func (b *BlobStore) readBlobData(keys []*datastore.Key, dst interface{}) error {
	ctx := context.TODO()
	var egroup errgroup.Group
	blobParts := make([]*blobFrag, len(keys))
	for idx, key := range keys {
		func(idx int, key *datastore.Key) {
			egroup.Go(func() error {
				blobParts[idx] = &blobFrag{}
				if err := b.client.Get(ctx, key, blobParts[idx]); err != nil {
					return sklog.FmtErrorf("Error loading blob data %s", err)
				}
				return nil
			})
		}(idx, key)
	}
	if err := egroup.Wait(); err != nil {
		return err
	}

	if err := jsonDecodeBlobParts(blobParts, dst); err != nil {
		return sklog.FmtErrorf("Error decoding JSON from blobs: %s", err)
	}
	return nil
}

// jsonEncodeBlobParts encodes the given object to a JSON and spreads it over a
// sequence of blobFrag instances that are within the limits of entities stored
// in cloud datastore.
func jsonEncodeBlobParts(src interface{}) ([]*blobFrag, error) {
	var buf bytes.Buffer
	if err := json.NewEncoder(&buf).Encode(src); err != nil {
		return nil, err
	}
	body := buf.Bytes()

	nPages := int(math.Ceil(float64(len(body)) / float64(maxBlobFragBytes)))

	ret := make([]*blobFrag, nPages)
	for p := 0; p < nPages; p++ {
		start := p * maxBlobFragBytes
		ret[p] = &blobFrag{
			Body: body[start:util.MinInt(start+maxBlobFragBytes, len(body))],
		}
	}
	return ret, nil
}

// jsonDecodeBlobParts decodes the JSON contained in the sequence of blobPart
// instances.
func jsonDecodeBlobParts(parts []*blobFrag, dst interface{}) error {
	var buf bytes.Buffer
	for _, part := range parts {
		if _, err := buf.Write(part.Body); err != nil {
			return err
		}
	}
	return json.NewDecoder(&buf).Decode(dst)
}
