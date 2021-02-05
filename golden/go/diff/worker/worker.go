// Package worker contains code that will compute the diffs for PNG images and write them to the
// SQL database. There could be multiple of these workers running in parallel to make sure the
// frontend can quickly make queries based on how close images are to one another.
package worker

import (
	"bytes"
	"context"
	"encoding/hex"
	"image"
	"image/png"
	"sort"
	"sync"
	"time"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4/pgxpool"
	ttlcache "github.com/patrickmn/go-cache"
	"go.opencensus.io/trace"
	"golang.org/x/sync/errgroup"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

const (
	// NowSourceKey is the context key used for the time source. If not provided, time.Now() will
	// be used.
	NowSourceKey = contextKey("nowSource")

	// 10k encoded images at ~ 1MB a piece = 10 gig of RAM, which should be doable.
	downloadedImageCacheSize = 10_000

	diffingRoutines = 8
)

var (
	// In an effort to prevent spamming the ProblemImages database, we skip known bad images for
	// a period of time. This time is controlled by badImageCooldown and a TTL cache.
	badImageCooldown = time.Minute
)

type contextKey string

// NowSource is an abstraction around a clock.
type NowSource interface {
	Now() time.Time
}

// ImageSource is an abstraction around a way to load the images. If images are stored in GCS, or
// on a file system or wherever, they should be provided by this mechanism.
type ImageSource interface {
	// GetImage returns the raw bytes of an image with the corresponding Digest.
	GetImage(ctx context.Context, digest types.Digest) ([]byte, error)
}

// WorkerImpl is a basic implementation that reads and writes to the SQL backend.
type WorkerImpl struct {
	db                   *pgxpool.Pool
	imageSource          ImageSource
	badDigestsCache      *ttlcache.Cache
	downloadedImageCache *lru.Cache
	// TODO(kjlubick) this might not be the best parameter for "digests to compute against" as we
	//   might just want to query for the last N commits with data and start at that tile to better
	//   handle the sparse data case.
	tilesToProcess           int
	metricsCalculatedCounter metrics2.Counter
	decodedImageBytesSummary metrics2.Float64SummaryMetric
	encodedImageBytesSummary metrics2.Float64SummaryMetric
}

// New returns a WorkerImpl that is ready to compute diffs.
func New(db *pgxpool.Pool, src ImageSource, tilesToProcess int) *WorkerImpl {
	imgCache, err := lru.New(downloadedImageCacheSize)
	if err != nil {
		panic(err) // should only happen if provided size is negative.
	}
	return &WorkerImpl{
		db:                       db,
		imageSource:              src,
		badDigestsCache:          ttlcache.New(badImageCooldown, 2*badImageCooldown),
		downloadedImageCache:     imgCache,
		tilesToProcess:           tilesToProcess,
		metricsCalculatedCounter: metrics2.GetCounter("diffcalculator_metricscalculated"),
		decodedImageBytesSummary: metrics2.GetFloat64SummaryMetric("diffcalculator_decodedimagebytes"),
		encodedImageBytesSummary: metrics2.GetFloat64SummaryMetric("diffcalculator_encodedimagebytes"),
	}
}

type digestPair struct {
	left  types.Digest
	right types.Digest
}

// CalculateDiffs calculates all diffmetrics for the current grouping, including any digests
// provided. It will not recalculate existing metrics, which are assumed to be immutable over time.
func (w *WorkerImpl) CalculateDiffs(ctx context.Context, grouping paramtools.Params, additional []types.Digest) error {
	ctx, span := trace.StartSpan(ctx, "CalculateDiffs")
	defer span.End()
	allDigests, err := w.getExisting(ctx, grouping)
	if err != nil {
		return skerr.Wrapf(err, "getting existing for %v", grouping)
	}
	allDigests = append(allDigests, additional...)
	sort.Sort(types.DigestSlice(allDigests))

	// Enumerate all the possible diffs that might need to be computed.
	toCompute := map[digestPair]bool{}
	for i := range allDigests {
		for j := i + 1; j < len(allDigests); j++ {
			toCompute[digestPair{left: allDigests[i], right: allDigests[j]}] = true
		}
	}
	if err := w.removeAlreadyComputed(ctx, toCompute, allDigests); err != nil {
		return skerr.Wrapf(err, "checking existing diff metrics")
	}

	sCtx, computeSpan := trace.StartSpan(ctx, "computeDiffsInParallel")
	// Compute diffs in parallel
	work := make(chan digestPair, len(toCompute))
	var newMetrics []schema.DiffMetricRow
	newMutex := sync.Mutex{}
	eg, eCtx := errgroup.WithContext(sCtx)
	for i := 0; i < diffingRoutines; i++ {
		eg.Go(func() error {
			for pair := range work {
				// If either image is known to be bad, skip this work
				_, bad1 := w.badDigestsCache.Get(string(pair.left))
				_, bad2 := w.badDigestsCache.Get(string(pair.right))
				if bad1 || bad2 {
					continue
				}
				nm, iErr := w.diff(eCtx, pair.left, pair.right)
				// If there is an error diffing, it is because we couldn't download or decode
				// one of the images. If so, we skip that entry and report it, before moving on.
				if iErr != nil {
					// Add the bad image to the ttlcache so we skip it for a short while.
					w.badDigestsCache.Set(string(iErr.digest), true, ttlcache.DefaultExpiration)
					if err := w.reportProblemImage(eCtx, iErr); err != nil {
						return skerr.Wrap(err)
					}
					continue
				}
				newMutex.Lock()
				newMetrics = append(newMetrics, nm)
				newMutex.Unlock()
			}
			return nil
		})
	}
	workAssigned := 0
	for pair, shouldCompute := range toCompute {
		if !shouldCompute {
			continue
		}
		workAssigned++
		work <- pair
	}
	close(work)
	sklog.Infof("Computing %d new diffs for grouping %v (%d diffs total)", workAssigned, grouping, len(toCompute))
	if err := eg.Wait(); err != nil {
		return skerr.Wrap(err)
	}
	computeSpan.End()
	// These numbers can be different in the case of errors.
	sklog.Infof("Done with those %d/%d new diffs", len(newMetrics), workAssigned)
	if len(newMetrics) == 0 {
		return nil
	}
	if err := w.writeMetrics(ctx, newMetrics); err != nil {
		return skerr.Wrapf(err, "writing %d new metrics for grouping %v", len(newMetrics), grouping)
	}
	w.metricsCalculatedCounter.Inc(int64(len(newMetrics)))
	return nil
}

// getExisting returns the unique digests that were seen on the primary branch for a given grouping
// over the most recent N tiles.
func (w *WorkerImpl) getExisting(ctx context.Context, grouping paramtools.Params) ([]types.Digest, error) {
	ctx, span := trace.StartSpan(ctx, "getExisting")
	defer span.End()
	row := w.db.QueryRow(ctx, `SELECT max(commit_id) FROM Commits
AS OF SYSTEM TIME '-0.1s'
WHERE has_data = TRUE`)
	var lc pgtype.Int4
	if err := row.Scan(&lc); err != nil {
		return nil, skerr.Wrapf(err, "getting latest commit")
	}
	if lc.Status == pgtype.Null {
		// There are no commits with data
		return nil, nil
	}
	latestCommit := schema.CommitID(lc.Int)
	currentTileStart := sql.ComputeTileStartID(latestCommit, schema.TileWidth)
	// Go backwards so we can use the current tile plus the previous n-1 tiles.
	beginTileStart := currentTileStart - schema.CommitID(schema.TileWidth*(w.tilesToProcess-1))
	_, groupingID := sql.SerializeMap(grouping)

	const statement = `
WITH
TracesMatchingGrouping AS (
  SELECT trace_id FROM Traces WHERE grouping_id = $1
)
SELECT DISTINCT digest FROM TiledTraceDigests
JOIN TracesMatchingGrouping on TiledTraceDigests.trace_id = TracesMatchingGrouping.trace_id
WHERE TiledTraceDigests.start_commit_id >= $2`

	rows, err := w.db.Query(ctx, statement, groupingID, beginTileStart)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching digests")
	}
	defer rows.Close()
	var rv []types.Digest
	for rows.Next() {
		var d schema.DigestBytes
		if err := rows.Scan(&d); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, types.Digest(hex.EncodeToString(d)))
	}
	return rv, nil
}

// removeAlreadyComputed will query the SQL database for DiffMetrics that have already been computed
// and will mark those entries in the provided map as false, signalling that they should not be
// recomputed.
func (w *WorkerImpl) removeAlreadyComputed(ctx context.Context, toCompute map[digestPair]bool, digests []types.Digest) error {
	ctx, span := trace.StartSpan(ctx, "removeAlreadyComputed")
	defer span.End()
	if len(digests) == 0 || len(toCompute) == 0 {
		return nil
	}
	const statement = `
SELECT encode(left_digest, 'hex'), encode(right_digest, 'hex') FROM DiffMetrics
AS OF SYSTEM TIME '-0.1s'
WHERE left_digest IN `
	vp := sql.ValuesPlaceholders(len(digests), 1)
	arguments := make([]interface{}, 0, len(digests))
	for _, d := range digests {
		b, err := sql.DigestToBytes(d)
		if err != nil {
			return skerr.Wrap(err)
		}
		arguments = append(arguments, b)
	}
	rows, err := w.db.Query(ctx, statement+vp, arguments...)
	if err != nil {
		return skerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		var left types.Digest
		var right types.Digest
		if err := rows.Scan(&left, &right); err != nil {
			return skerr.Wrap(err)
		}
		// Mark this entry false to indicate we've already seen it.
		toCompute[digestPair{left: left, right: right}] = false
	}
	return nil
}

type imgError struct {
	err    error
	digest types.Digest
}

// diff calculates the difference between the two images with the provided digests and returns
// it in a format that can be inserted into the SQL database. If there is an error downloading
// or decoding a digest, an error is returned along with the problematic digest.
func (w *WorkerImpl) diff(ctx context.Context, left, right types.Digest) (schema.DiffMetricRow, *imgError) {
	ctx, span := trace.StartSpan(ctx, "diff")
	defer span.End()
	lb, err := sql.DigestToBytes(left)
	if err != nil {
		return schema.DiffMetricRow{}, &imgError{digest: left, err: skerr.Wrap(err)}
	}
	rb, err := sql.DigestToBytes(right)
	if err != nil {
		return schema.DiffMetricRow{}, &imgError{digest: right, err: skerr.Wrap(err)}
	}
	leftImg, err := w.getDecodedImage(ctx, left)
	if err != nil {
		return schema.DiffMetricRow{}, &imgError{digest: left, err: skerr.Wrap(err)}
	}
	rightImg, err := w.getDecodedImage(ctx, right)
	if err != nil {
		return schema.DiffMetricRow{}, &imgError{digest: right, err: skerr.Wrap(err)}
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
	ctx, span := trace.StartSpan(ctx, "getDecodedImage")
	defer span.End()
	if cachedBytes, ok := w.downloadedImageCache.Get(digest); ok {
		return decode(ctx, cachedBytes.([]byte))
	}
	b, err := w.imageSource.GetImage(ctx, digest)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting image with digest %s", digest)
	}
	w.downloadedImageCache.Add(digest, b)
	w.encodedImageBytesSummary.Observe(float64(len(b)))
	img, err := decode(ctx, b)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// In memory, the image takes up 4 bytes per pixel.
	s := img.Bounds().Size()
	w.decodedImageBytesSummary.Observe(float64(s.X * s.Y * 4))
	return img, nil
}

// writeMetrics writes two copies of the provided metrics (one for left-right and one for
// right-left) to the SQL database.
func (w *WorkerImpl) writeMetrics(ctx context.Context, metrics []schema.DiffMetricRow) error {
	ctx, span := trace.StartSpan(ctx, "writeMetrics")
	defer span.End()
	const baseStatement = `UPSERT INTO DiffMetrics
(left_digest, right_digest, num_pixels_diff, percent_pixels_diff, max_rgba_diffs,
max_channel_diff, combined_metric, dimensions_differ, ts) VALUES `
	const valuesPerRow = 9
	// This batchSize picked somewhat arbitrarily.
	const batchSize = 100
	arguments := make([]interface{}, 0, batchSize*valuesPerRow*2)
	return skerr.Wrap(util.ChunkIter(len(metrics), batchSize, func(startIdx int, endIdx int) error {
		arguments = arguments[:0] // clear out previous arguments
		count := 0
		for _, r := range metrics[startIdx:endIdx] {
			count += 2
			rgba := make([]int, 4)
			copy(rgba, r.MaxRGBADiffs[:])
			arguments = append(arguments, r.LeftDigest, r.RightDigest, r.NumPixelsDiff, r.PercentPixelsDiff, rgba,
				r.MaxChannelDiff, r.CombinedMetric, r.DimensionsDiffer, r.Timestamp)
			arguments = append(arguments, r.RightDigest, r.LeftDigest, r.NumPixelsDiff, r.PercentPixelsDiff, rgba,
				r.MaxChannelDiff, r.CombinedMetric, r.DimensionsDiffer, r.Timestamp)
		}
		vp := sql.ValuesPlaceholders(valuesPerRow, count)
		_, err := w.db.Exec(ctx, baseStatement+vp, arguments...)
		return skerr.Wrap(err)
	}))
}

// reportProblemImage creates or updates a row in the ProblemImages table for the given digest.
func (w *WorkerImpl) reportProblemImage(ctx context.Context, imgErr *imgError) error {
	ctx, span := trace.StartSpan(ctx, "reportProblemImage")
	defer span.End()
	sklog.Warningf("Reporting problem with image %s: %s", imgErr.digest, imgErr.err)
	const statement = `
INSERT INTO ProblemImages (digest, num_errors, latest_error, error_ts)
VALUES ($1, $2, $3, $4)
ON CONFLICT (digest)
DO UPDATE SET (num_errors, latest_error, error_ts) =
(ProblemImages.num_errors + 1, $3, $4)`

	_, err := w.db.Exec(ctx, statement, imgErr.digest, 1, imgErr.err.Error(), now(ctx))
	if err != nil {
		return skerr.Wrapf(err, "writing to ProblemImages")
	}
	return nil
}

// decode decodes the provided bytes as a PNG and returns them.
func decode(ctx context.Context, b []byte) (*image.NRGBA, error) {
	ctx, span := trace.StartSpan(ctx, "decode")
	defer span.End()
	im, err := png.Decode(bytes.NewReader(b))
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return diff.GetNRGBA(im), nil
}

// Make sure WorkerImpl fulfills the diff.Calculator interface.
var _ diff.Calculator = (*WorkerImpl)(nil)
