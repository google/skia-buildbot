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
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/types"
)

const (
	// NowSourceKey is the context key used for the time source. If not provided, time.Now() will
	// be used.
	NowSourceKey = contextKey("nowSource")

	// 2k decoded images at ~5MB a piece = 10 gig of RAM, which should be doable.
	// The size of 5MB comes from the 90th percentile of real-world data.
	decodedImageCacheSize = 2000

	diffingRoutines = 4

	// This batch size is picked arbitrarily.
	reportingBatchSize = 100
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
	db                *pgxpool.Pool
	imageSource       ImageSource
	badDigestsCache   *ttlcache.Cache
	decodedImageCache *lru.Cache
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
	imgCache, err := lru.New(decodedImageCacheSize)
	if err != nil {
		panic(err) // should only happen if provided size is negative.
	}
	return &WorkerImpl{
		db:                       db,
		imageSource:              src,
		badDigestsCache:          ttlcache.New(badImageCooldown, 2*badImageCooldown),
		decodedImageCache:        imgCache,
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

// newDigestPair returns a digestPair in a "canonical" order, such that left < right. This avoids
// effective duplicates (since comparing left to right is the same right to left).
func newDigestPair(one, two types.Digest) digestPair {
	if one < two {
		return digestPair{left: one, right: two}
	}
	return digestPair{left: two, right: one}
}

// CalculateDiffs calculates all diffmetrics for the current grouping, including any digests
// provided. It will not recalculate existing metrics, which are assumed to be immutable over time.
// Digests from all traces will be included in the left bucket, digests from not ignored traces
// will be included in the right bucket.
func (w *WorkerImpl) CalculateDiffs(ctx context.Context, grouping paramtools.Params, addLeft, addRight []types.Digest) error {
	ctx, span := trace.StartSpan(ctx, "CalculateDiffs")
	defer span.End()
	startingTile, err := w.getStartingTile(ctx)
	if err != nil {
		return skerr.Wrapf(err, "get starting tile")
	}

	leftDigests, err := w.getAllExisting(ctx, startingTile, grouping)
	if err != nil {
		return skerr.Wrapf(err, "getting all existing for %v", grouping)
	}
	leftDigests = append(leftDigests, addLeft...)

	rightDigests, err := w.getNonIgnoredExisting(ctx, startingTile, grouping)
	if err != nil {
		return skerr.Wrapf(err, "getting not-ignored existing for %v", grouping)
	}
	rightDigests = append(rightDigests, addRight...)

	// Enumerate all the possible diffs that might need to be computed.
	toCompute := map[digestPair]bool{}
	for _, d1 := range leftDigests {
		for _, d2 := range rightDigests {
			if d1 != d2 {
				toCompute[newDigestPair(d1, d2)] = true
			}
		}
	}
	// We expect leftDigests to be a superset of rightDigest
	if err := w.removeAlreadyComputed(ctx, toCompute, leftDigests); err != nil {
		return skerr.Wrapf(err, "checking existing diff metrics")
	}

	sCtx, computeSpan := trace.StartSpan(ctx, "computeAndReportDiffsInParallel")
	defer computeSpan.End()
	// Compute diffs in parallel. To do this, we spin up an error group that uses a channel to
	// distribute work and an error group that listens to the output channel and uploads in batches.
	// This way, we have some way to stream results and the metrics will be smoother in case a lot
	// of computations need to happen over the course of several minutes.
	availableWork := make(chan digestPair, len(toCompute))
	// We expect pushing to SQL to be much faster than diffing, so we don't want the newMetrics
	// chan to be blocking computation. As such, make the buffer size decently big.
	newMetrics := make(chan schema.DiffMetricRow, 2*reportingBatchSize)
	computeGroup, eCtx := errgroup.WithContext(sCtx)
	for i := 0; i < diffingRoutines; i++ {
		computeGroup.Go(func() error {
			// Worker goroutines will run until the channel is empty and closed.
			for pair := range availableWork {
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
				newMetrics <- nm
			}
			return nil
		})
	}
	reportGroup, eCtx2 := errgroup.WithContext(sCtx)
	reportGroup.Go(func() error {
		// This goroutine will run until the newMetrics channel is empty and closed.
		buffer := make([]schema.DiffMetricRow, 0, reportingBatchSize)
		for nm := range newMetrics {
			buffer = append(buffer, nm)
			if len(buffer) >= reportingBatchSize {
				if err := w.writeMetrics(eCtx2, buffer); err != nil {
					return skerr.Wrap(err)
				}
				w.metricsCalculatedCounter.Inc(int64(len(buffer)))
				buffer = buffer[:0] // reset buffer
			}
		}
		if err := w.writeMetrics(eCtx2, buffer); err != nil {
			return skerr.Wrap(err)
		}
		w.metricsCalculatedCounter.Inc(int64(len(buffer)))
		return nil
	})
	// Now that the goroutines are started, fill up the availableWork channel and close it.
	workAssigned := 0
	for pair, shouldCompute := range toCompute {
		if !shouldCompute {
			continue
		}
		workAssigned++
		availableWork <- pair
	}
	close(availableWork)
	sklog.Infof("Computing %d new diffs for grouping %v (%d diffs total)", workAssigned, grouping, len(toCompute))
	// Wait for computation to complete.
	if err := computeGroup.Wait(); err != nil {
		close(newMetrics) // shut down the reporting go routine as well.
		return skerr.Wrap(err)
	}
	// We know computation is complete, so close the reporting channel and wait for it to be done
	// reporting the remaining metrics.
	close(newMetrics)
	if err := reportGroup.Wait(); err != nil {
		return skerr.Wrap(err)
	}
	sklog.Infof("Done with those %d new diffs", workAssigned)
	return nil
}

// getStartingTile returns the commit ID which is the beginning of the tile of interest (so we
// get enough data to do our comparisons).
func (w *WorkerImpl) getStartingTile(ctx context.Context) (schema.CommitID, error) {
	row := w.db.QueryRow(ctx, `SELECT max(commit_id) FROM Commits
AS OF SYSTEM TIME '-0.1s'
WHERE has_data = TRUE`)
	var lc pgtype.Int4
	if err := row.Scan(&lc); err != nil {
		return 0, skerr.Wrapf(err, "getting latest commit")
	}
	if lc.Status == pgtype.Null {
		// There are no commits with data
		return 0, nil
	}
	latestCommit := schema.CommitID(lc.Int)
	currentTileStart := sql.ComputeTileStartID(latestCommit, schema.TileWidth)
	// Go backwards so we can use the current tile plus the previous n-1 tiles.
	return currentTileStart - schema.CommitID(schema.TileWidth*(w.tilesToProcess-1)), nil
}

// getAllExisting returns the unique digests that were seen on the primary branch for a given
// grouping starting at the given commit.
func (w *WorkerImpl) getAllExisting(ctx context.Context, beginTileStart schema.CommitID, grouping paramtools.Params) ([]types.Digest, error) {
	ctx, span := trace.StartSpan(ctx, "getAllExisting")
	defer span.End()
	const statement = `
WITH
TracesMatchingGrouping AS (
  SELECT trace_id FROM Traces WHERE grouping_id = $1
)
SELECT DISTINCT digest FROM TiledTraceDigests
JOIN TracesMatchingGrouping on TiledTraceDigests.trace_id = TracesMatchingGrouping.trace_id
WHERE TiledTraceDigests.start_commit_id >= $2`

	_, groupingID := sql.SerializeMap(grouping)
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

func (w *WorkerImpl) getNonIgnoredExisting(ctx context.Context, beginTileStart schema.CommitID, grouping paramtools.Params) ([]types.Digest, error) {
	ctx, span := trace.StartSpan(ctx, "getNonIgnoredExisting")
	defer span.End()
	const statement = `
WITH
NotIgnoredTracesMatchingGrouping AS (
  SELECT trace_id FROM Traces WHERE grouping_id = $1 AND matches_any_ignore_rule = FALSE
)
SELECT DISTINCT digest FROM TiledTraceDigests
JOIN NotIgnoredTracesMatchingGrouping on TiledTraceDigests.trace_id = NotIgnoredTracesMatchingGrouping.trace_id
WHERE TiledTraceDigests.start_commit_id >= $2`

	_, groupingID := sql.SerializeMap(grouping)
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
		toCompute[newDigestPair(left, right)] = false
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
	if cachedImg, ok := w.decodedImageCache.Get(digest); ok {
		return cachedImg.(*image.NRGBA), nil
	}
	b, err := w.imageSource.GetImage(ctx, digest)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting image with digest %s", digest)
	}
	w.encodedImageBytesSummary.Observe(float64(len(b)))
	img, err := decode(ctx, b)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// In memory, the image takes up 4 bytes per pixel.
	s := img.Bounds().Size()
	w.decodedImageBytesSummary.Observe(float64(s.X * s.Y * 4))
	w.decodedImageCache.Add(digest, img)
	return img, nil
}

// writeMetrics writes two copies of the provided metrics (one for left-right and one for
// right-left) to the SQL database.
func (w *WorkerImpl) writeMetrics(ctx context.Context, metrics []schema.DiffMetricRow) error {
	if len(metrics) == 0 {
		return nil
	}
	ctx, span := trace.StartSpan(ctx, "writeMetrics")
	defer span.End()
	const baseStatement = `UPSERT INTO DiffMetrics
(left_digest, right_digest, num_pixels_diff, percent_pixels_diff, max_rgba_diffs,
max_channel_diff, combined_metric, dimensions_differ, ts) VALUES `
	const valuesPerRow = 9

	arguments := make([]interface{}, 0, len(metrics)*valuesPerRow*2)
	count := 0
	for _, r := range metrics {
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
	if err != nil {
		return skerr.Wrapf(err, "writing %d metrics to SQL", len(metrics))
	}
	return nil
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
