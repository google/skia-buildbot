package worker

import (
	"bytes"
	"context"
	"image"
	"image/png"
	"sort"
	"time"

	"go.skia.org/infra/go/util"

	"go.skia.org/infra/golden/go/sql"

	"go.skia.org/infra/golden/go/diff"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgx/v4/pgxpool"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

const (
	// NowSourceKey is the context key used for the time source. If not provided, time.Now() will
	// be used.
	NowSourceKey = contextKey("nowSource")

	// 10k decoded images at ~ 1MB a piece = 10 gig of RAM, which should be doable.
	downloadedImageCacheSize = 10_000
)

type contextKey string

type NowSource interface {
	Now() time.Time
}

type ImageSource interface {
	// GetImage returns the raw bytes of an image with the corresponding Digest.
	GetImage(ctx context.Context, digest types.Digest) ([]byte, error)
}

type WorkerImpl struct {
	db                   *pgxpool.Pool
	imageSource          ImageSource
	downloadedImageCache *lru.Cache
}

func New(db *pgxpool.Pool, src ImageSource) *WorkerImpl {
	imgCache, err := lru.New(downloadedImageCacheSize)
	if err != nil {
		panic(err) // should only happen if provided size is negative.
	}
	return &WorkerImpl{db: db, imageSource: src, downloadedImageCache: imgCache}
}

type digestPair struct {
	left  types.Digest
	right types.Digest
}

// ComputeDiffs recomputes all diffs for the current grouping, including any digests provided.
func (w *WorkerImpl) ComputeDiffs(ctx context.Context, grouping paramtools.Params, additional []types.Digest) error {
	// TODO(kjlubick) fetch existing digests from DB
	allDigests := additional

	sort.Sort(types.DigestSlice(allDigests))

	toCompute := map[digestPair]bool{}
	for i := range allDigests {
		for j := i + 1; j < len(allDigests); j++ {
			toCompute[digestPair{left: allDigests[i], right: allDigests[j]}] = true
		}
	}
	// TODO(kjlubick) fetch existing diff metrics from DB for grouping

	// TODO(kjlubick) run this in parallel with a channel and goroutines
	var newMetrics []schema.DiffMetricRow
	for pair, shouldCompute := range toCompute {
		if !shouldCompute {
			continue
		}
		nm, err := w.diff(ctx, pair.left, pair.right)
		if err != nil {
			return skerr.Wrapf(err, "diffing %s and %s", pair.left, pair.right)
		}
		newMetrics = append(newMetrics, nm)
	}

	if err := w.writeMetrics(ctx, newMetrics); err != nil {
		return skerr.Wrapf(err, "writing %d new metrics for grouping %v", len(newMetrics), grouping)
	}
	return nil
}

// diff computes the difference between the two images with the provided digests and returns
// it in a format that can be inserted into the SQL database.
func (w *WorkerImpl) diff(ctx context.Context, left types.Digest, right types.Digest) (schema.DiffMetricRow, error) {
	lb, err := sql.DigestToBytes(left)
	if err != nil {
		return schema.DiffMetricRow{}, skerr.Wrap(err)
	}
	rb, err := sql.DigestToBytes(right)
	if err != nil {
		return schema.DiffMetricRow{}, skerr.Wrap(err)
	}
	leftImg, err := w.getDecodedImage(ctx, left)
	if err != nil {
		return schema.DiffMetricRow{}, skerr.Wrap(err)
	}
	rightImg, err := w.getDecodedImage(ctx, right)
	if err != nil {
		return schema.DiffMetricRow{}, skerr.Wrap(err)
	}
	m := diff.ComputeDiffMetrics(leftImg, rightImg)
	return schema.DiffMetricRow{
		LeftDigest:        lb,
		RightDigest:       rb,
		NumPixelsDiff:     m.NumDiffPixels,
		PercentPixelsDiff: m.PixelDiffPercent,
		MaxRGBADiffs:      m.MaxRGBADiffs,
		MaxChannelDiff:    max(m.MaxRGBADiffs),
		CombinedMetric:    m.CombinedMetric,
		DimensionsDiffer:  m.DimDiffer,
		Timestamp:         now(ctx),
	}, nil
}

func max(diffs [4]int) int {
	m := diffs[0]
	for _, d := range diffs {
		if d > m {
			m = d
		}
	}
	return m
}

// now returns the current time, using a provided NowSource on the context if available.
func now(ctx context.Context) time.Time {
	ns, ok := ctx.Value(NowSourceKey).(NowSource)
	if ns == nil || !ok {
		return time.Now()
	}
	return ns.Now()
}

// getImage retrieves and decodes the given image. If the image is cached, this function will
// return the cached version.
func (w *WorkerImpl) getDecodedImage(ctx context.Context, digest types.Digest) (*image.NRGBA, error) {
	if cachedBytes, ok := w.downloadedImageCache.Get(digest); ok {
		return decode(cachedBytes.([]byte))
	}
	b, err := w.imageSource.GetImage(ctx, digest)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting image with digest %s", digest)
	}
	w.downloadedImageCache.Add(digest, b)
	return decode(b)
}

// writeMetrics writes two copies of the provided metrics (one for left-right and one for
// right-left) to the SQL database.
func (w *WorkerImpl) writeMetrics(ctx context.Context, metrics []schema.DiffMetricRow) error {
	const baseStatement = `UPSERT INTO DiffMetrics
(left_digest, right_digest, num_pixels_diff, percent_pixels_diff, max_rgba_diffs,
max_channel_diff, combined_metric, dimensions_differ, ts) VALUES `
	const batchSize = 100
	const valuesPerRow = 9
	arguments := make([]interface{}, 0, batchSize*valuesPerRow*2)
	return skerr.Wrap(util.ChunkIter(len(metrics), batchSize, func(startIdx int, endIdx int) error {
		arguments = arguments[:0] // clear out previous arguments
		count := 0
		for _, r := range metrics[startIdx:endIdx] {
			count += 2
			arguments = append(arguments, r.LeftDigest, r.RightDigest, r.NumPixelsDiff, r.PercentPixelsDiff, r.MaxRGBADiffs,
				r.MaxChannelDiff, r.CombinedMetric, r.DimensionsDiffer, r.Timestamp)
			arguments = append(arguments, r.RightDigest, r.LeftDigest, r.NumPixelsDiff, r.PercentPixelsDiff, r.MaxRGBADiffs,
				r.MaxChannelDiff, r.CombinedMetric, r.DimensionsDiffer, r.Timestamp)
		}
		vp := sql.ValuesPlaceholders(valuesPerRow, count)
		_, err := w.db.Exec(ctx, baseStatement+vp, arguments...)
		return skerr.Wrap(err)
	}))
}

// decode decodes the provided bytes as a PNG and returns them.
func decode(b []byte) (*image.NRGBA, error) {
	im, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return diff.GetNRGBA(im), nil
}
