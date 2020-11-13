package fs_metricsstore

import (
	"context"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/metricsstore"
	"go.skia.org/infra/golden/go/types"
)

const (
	// Firestore collection name.
	metricsStoreCollection = "metricsstore_diffmetrics"

	maxReadAttempts  = 5
	maxWriteAttempts = 5
	maxOperationTime = time.Minute

	// Fields we query by.
	leftAndRightDigests = "left_and_right_digests"
)

// StoreImpl is the Firestore-backed implementation of MetricsStore.
type StoreImpl struct {
	client *ifirestore.Client
}

// storeEntry represents how a diff.DiffMetrics instance is stored in Firestore.
type storeEntry struct {
	LeftAndRightDigests []string `firestore:"left_and_right_digests"` // Needed to support purging by digest ID.
	NumDiffPixels       int      `firestore:"num_diff_pixels"`
	PercentDiffPixels   float32  `firestore:"percent_diff_pixels"`
	MaxRGBADiffs        [4]int   `firestore:"max_rgba_diffs"`
	DimensionsDiffer    bool     `firestore:"dimensions_differ"`
}

// toDiffMetrics converts a storeEntry into a diff.DiffMetrics instance. It sets the Diffs map the
// same way as ComputeDiffMetrics does.
func (e *storeEntry) toDiffMetrics() *diff.DiffMetrics {
	diffMetrics := &diff.DiffMetrics{
		NumDiffPixels:    e.NumDiffPixels,
		PixelDiffPercent: e.PercentDiffPixels,
		MaxRGBADiffs:     [4]int{e.MaxRGBADiffs[0], e.MaxRGBADiffs[1], e.MaxRGBADiffs[2], e.MaxRGBADiffs[3]},
		DimDiffer:        e.DimensionsDiffer,
	}
	diffMetrics.CombinedMetric = diff.CombinedDiffMetric(e.MaxRGBADiffs, e.PercentDiffPixels)
	return diffMetrics
}

// setLeftAndRightDigests sets the LeftAndRightDigests field based on the given diff id.
func (e *storeEntry) setLeftAndRightDigests(id string) {
	e.LeftAndRightDigests = strings.Split(id, common.DiffImageSeparator)
}

// toStoreEntry converts a diff.DiffMetrics instance into a storeEntry. It assumes the given
// diff.DiffMetrics instance was generated with ComputeDiffMetrics(), which computes the Diffs field
// from the other fields in the struct, and therefore is not necessary to store in Firestore.
func toStoreEntry(dm *diff.DiffMetrics) storeEntry {
	return storeEntry{
		NumDiffPixels:     dm.NumDiffPixels,
		PercentDiffPixels: dm.PixelDiffPercent,
		MaxRGBADiffs:      [4]int{dm.MaxRGBADiffs[0], dm.MaxRGBADiffs[1], dm.MaxRGBADiffs[2], dm.MaxRGBADiffs[3]},
		DimensionsDiffer:  dm.DimDiffer,
	}
}

// New returns a new instance of the Firestore-backed metricsstore.MetricsStore
// implementation.
func New(client *ifirestore.Client) *StoreImpl {
	return &StoreImpl{
		client: client,
	}
}

// PurgeDiffMetrics implements the metricsstore.MetricsStore interface.
func (s *StoreImpl) PurgeDiffMetrics(ctx context.Context, digests types.DigestSlice) error {
	defer metrics2.FuncTimer().Stop()

	// Find all matching documents by building one query per digest.
	queries := []firestore.Query{}
	for _, digest := range digests {
		// Using array-contains here probably implies a performance hit, but since purging is a very
		// infrequent operation, this is probably OK.
		q := s.client.Collection(metricsStoreCollection).Where(leftAndRightDigests, "array-contains", string(digest))
		queries = append(queries, q)
	}

	// The queries above could potentially return the same document multiple times; e.g. a document
	// with ID "abc-def" will be matched twice if the digests slice contains both "abc" and "def".
	//
	// For this reason, we will store the document IDs in a string->bool map (i.e. a set), which will
	// later guarantee that each document is deleted only once.
	docsToPurge := sync.Map{}

	// Iterate over query results in parallel, and populate the map above.
	f := func(_ int, doc *firestore.DocumentSnapshot) error {
		docsToPurge.Store(doc.Ref.ID, true)
		return nil
	}
	if err := s.client.IterDocsInParallel(ctx, "PurgeDiffMetrics", "", queries, maxReadAttempts, maxOperationTime, f); err != nil {
		return skerr.Wrap(err)
	}

	// Delete documents one by one.
	var err error
	var errDocID string
	docsToPurge.Range(func(key, _ interface{}) bool {
		docID := key.(string)
		// We delete documents one by one instead of using a WriteBatch because batches are limited to
		// 500 writes. Given how infrequent the purge operation is, this slower approach is probably OK.
		if _, err = s.client.Collection(metricsStoreCollection).Doc(docID).Delete(ctx); err != nil {
			errDocID = docID
			return false
		}
		return true
	})

	// Check for errors and return.
	if err != nil {
		return skerr.Wrapf(err, "deleting document with ID %q", errDocID)
	}
	return nil
}

// SaveDiffMetrics implements the metricsstore.MetricsStore interface.
func (s *StoreImpl) SaveDiffMetrics(ctx context.Context, id string, diffMetrics *diff.DiffMetrics) error {
	defer metrics2.FuncTimer().Stop()

	docRef := s.client.Collection(metricsStoreCollection).Doc(id)
	entry := toStoreEntry(diffMetrics)
	entry.setLeftAndRightDigests(id)
	if _, err := s.client.Set(ctx, docRef, entry, maxWriteAttempts, maxOperationTime); err != nil {
		return skerr.Wrapf(err, "writing diff metrics to Firestore (ID=%q): %v", id, diffMetrics)
	}
	return nil
}

// LoadDiffMetrics implements the metricsstore.MetricsStore interface.
func (s *StoreImpl) LoadDiffMetrics(ctx context.Context, ids []string) ([]*diff.DiffMetrics, error) {
	defer metrics2.FuncTimer().Stop()

	xDoc := make([]*firestore.DocumentRef, 0, len(ids))
	for _, id := range ids {
		xDoc = append(xDoc, s.client.Collection(metricsStoreCollection).Doc(id))
	}

	s.client.CountReadQueryAndRows(s.client.Collection(metricsStoreCollection).Path, len(xDoc))
	xds, err := s.client.GetAll(ctx, xDoc)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	rv := make([]*diff.DiffMetrics, len(ids))
	for i, doc := range xds {
		// doc.Exists() is false if the entry didn't exist in Firestore.
		// The nil check is to be paranoid.
		if doc == nil || !doc.Exists() {
			continue
		}

		// Unmarshal data.
		entry := storeEntry{}
		if err := doc.DataTo(&entry); err != nil {
			id := doc.Ref.ID
			return nil, skerr.Wrapf(err, "corrupt data in Firestore, could not unmarshal metrics with id %s", id)
		}

		rv[i] = entry.toDiffMetrics()
	}

	return rv, nil
}

// Make sure StoreImpl fulfills the MetricsStore interface
var _ metricsstore.MetricsStore = (*StoreImpl)(nil)
