package fs_metricsstore

import (
	"context"
	"strings"
	"sync"
	"time"

	"cloud.google.com/go/firestore"
	ifirestore "go.skia.org/infra/go/firestore"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/diffstore/common"
	"go.skia.org/infra/golden/go/diffstore/metricsstore"
	"go.skia.org/infra/golden/go/types"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const (
	// Firestore collection name.
	// TODO(lovisolo): "metrics" is a bit ambiguous, even within the context of Gold. Should we call
	//                 this "diffmetrics"? And possibly rename MetricsStore to DiffMetricsStore.
	metricsStoreCollection = "diffmetrics"

	maxReadAttempts  = 5
	maxWriteAttempts = 5
	maxOperationTime = time.Minute
)

// storeImpl is the Firestore-backed implementation of MetricsStore.
type storeImpl struct {
	client *ifirestore.Client
}

// storeEntry represents how a diff.DiffMetrics instance is stored in Firestore.
type storeEntry struct {
	LeftAndRightDigests []string `firestore:"left_and_right_digests"` // Needed to support the PurgeDiffMetrics method.
	NumDiffPixels       int      `firestore:"num_diff_pixels"`
	PercentDiffPixels   float32  `firestore:"percent_diff_pixels"`
	MaxDiffR            int      `firestore:"max_diff_r"`
	MaxDiffG            int      `firestore:"max_diff_g"`
	MaxDiffB            int      `firestore:"max_diff_b"`
	MaxDiffA            int      `firestore:"max_diff_a"`
	DimensionsDiffer    bool     `firestore:"dimensions_differ"`
}

// toDiffMetrics converts a storeEntry into a diff.DiffMetrics instance. It sets the Diffs map the
// same way as DefaultDiffFn does.
func (e *storeEntry) toDiffMetrics() *diff.DiffMetrics {
	diffMetrics := &diff.DiffMetrics{
		NumDiffPixels:    e.NumDiffPixels,
		PixelDiffPercent: e.PercentDiffPixels,
		MaxRGBADiffs:     []int{e.MaxDiffR, e.MaxDiffG, e.MaxDiffB, e.MaxDiffA},
		DimDiffer:        e.DimensionsDiffer,
		Diffs: map[string]float32{
			// TODO(lovisolo): Reuse functions (percent,pixel)DiffMetric in metrics.go here.
			diff.METRIC_PERCENT: e.PercentDiffPixels,
			diff.METRIC_PIXEL:   float32(e.NumDiffPixels),
		},
	}
	diffMetrics.Diffs[diff.METRIC_COMBINED] = diff.CombinedDiffMetric(diffMetrics, nil, nil)
	return diffMetrics
}

// setLeftAndRightDigests sets the LeftAndRightDigests field based on the given diff id.
func (e *storeEntry) setLeftAndRightDigests(id string) {
	e.LeftAndRightDigests = strings.Split(id, common.DiffImageSeparator)
}

// toStoreEntry converts a diff.DiffMetrics instance into a storeEntry. It assumes the given
// diff.DiffMetrics instance was generated with DefaultDiffFn(), which computes the Diffs field
// from the other fields in the struct, and therefore is not necessary to store in Firestore.
func toStoreEntry(diffMetrics *diff.DiffMetrics) storeEntry {
	return storeEntry{
		NumDiffPixels:     diffMetrics.NumDiffPixels,
		PercentDiffPixels: diffMetrics.PixelDiffPercent,
		MaxDiffR:          diffMetrics.MaxRGBADiffs[0],
		MaxDiffG:          diffMetrics.MaxRGBADiffs[1],
		MaxDiffB:          diffMetrics.MaxRGBADiffs[2],
		MaxDiffA:          diffMetrics.MaxRGBADiffs[3],
		DimensionsDiffer:  diffMetrics.DimDiffer,
	}
}

// New returns a Firestore-backed instance of metricsstore.MetricsStore.
func New(client *ifirestore.Client) metricsstore.MetricsStore {
	return &storeImpl{
		client: client,
	}
}

// PurgeDiffMetrics implements the metricsstore.MetricsStore interface.
func (s *storeImpl) PurgeDiffMetrics(digests types.DigestSlice) error {
	ctx := context.Background() // TODO(lovisolo): Add a ctx argument to the interface method.

	// Find all matching documents by building one query per digest.
	queries := []firestore.Query{}
	for _, digest := range digests {
		q := s.client.Collection(metricsStoreCollection).Where("left_and_right_digests", "array-contains", string(digest))
		queries = append(queries, q)
	}

	// The queries above could potentially return the same document multiple times; e.g. a document
	// with ID "abc-def" will be matched twice if the digests slice contains both "abc" and "def".
	//
	// For this reason, we will store references to these documents in a map keyed by ID, which will
	// later guarantee that each document is deleted only once.
	docsToPurge := make(map[string]*firestore.DocumentRef)
	var docsToPurgeMux sync.Mutex

	// Iterate over query results in parallel, and populate the map above.
	f := func(_ int, doc *firestore.DocumentSnapshot) error {
		docsToPurgeMux.Lock()
		defer docsToPurgeMux.Unlock()
		docsToPurge[doc.Ref.ID] = doc.Ref
		return nil
	}
	if err := s.client.IterDocsInParallel("PurgeDiffMetrics", "", queries, maxReadAttempts, maxOperationTime, f); err != nil {
		return skerr.Wrap(err)
	}

	// Return if no documents match the given digests.
	if len(docsToPurge) == 0 {
		return nil
	}

	// Delete documents.
	wb := s.client.Batch()
	for _, docRef := range docsToPurge {
		wb = wb.Delete(docRef)
	}
	if _, err := wb.Commit(ctx); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}

// SaveDiffMetrics implements the metricsstore.MetricsStore interface.
func (s *storeImpl) SaveDiffMetrics(id string, diffMetrics *diff.DiffMetrics) error {
	docRef := s.client.Collection(metricsStoreCollection).Doc(id)
	entry := toStoreEntry(diffMetrics)
	entry.setLeftAndRightDigests(id)
	if _, err := s.client.Set(docRef, entry, maxWriteAttempts, maxOperationTime); err != nil {
		return skerr.Wrapf(err, "writing metrics to Firestore: %v", diffMetrics)
	}
	return nil
}

// LoadDiffMetrics implements the metricsstore.MetricsStore interface.
func (s *storeImpl) LoadDiffMetrics(id string) (*diff.DiffMetrics, error) {
	ctx := context.Background() // TODO(lovisolo): Add a ctx argument to the interface method.

	// Retrieve Firestore document.
	doc, err := s.client.Collection(metricsStoreCollection).Doc(id).Get(ctx)

	// Validate.
	if err != nil {
		if status.Code(err) == codes.NotFound {
			return nil, nil // Return nil if not found as per the Bolt-backed MetricsStore implementation.
		}
		return nil, skerr.Wrapf(err, "retrieving metrics from Firestore: %s", id)
	}
	if doc == nil {
		return nil, nil // Return nil if not found as per the Bolt-backed MetricsStore implementation.
	}

	// Unmarshal data.
	entry := storeEntry{}
	if err := doc.DataTo(&entry); err != nil {
		id := doc.Ref.ID
		return nil, skerr.Wrapf(err, "corrupt data in Firestore, could not unmarshal metrics with id %s", id)
	}

	// Convert to diff.DiffMetrics and return.
	return entry.toDiffMetrics(), nil
}
