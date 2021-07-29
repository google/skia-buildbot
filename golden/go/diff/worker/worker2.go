package worker

import (
	"context"
	"encoding/hex"
	"image"

	lru "github.com/hashicorp/golang-lru"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	ttlcache "github.com/patrickmn/go-cache"
	"go.opencensus.io/trace"

	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/now"
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
	// For tests with fewer than the computeTotalGridCutoff number of digests, we calculate all
	// diffs for the test. Above this number, we try to be more clever about calculating a subset
	// of useful diffs w/o doing too much work for very tests with very flaky traces.
	// This number was chosen because if there are 100 images, we would need to calculate
	// 100*99/2 = ~5k diffs; It can take up to .5 seconds to compute a diff (most of the time spent
	// decoding) and there are 4 worker goroutines. As such, we expect it to take 5k*.5/4 seconds
	// or about 600 seconds, which is the 10 minute timeout we use.
	computeTotalGridCutoff = 100
)

type WorkerImpl2 struct {
	db              *pgxpool.Pool
	imageSource     ImageSource
	badDigestsCache *ttlcache.Cache
	windowSize      int

	inputDigestsSummary      metrics2.Float64SummaryMetric
	digestsOfInterestSummary metrics2.Float64SummaryMetric
	metricsCalculatedCounter metrics2.Counter
}

// NewV2 returns a diff worker version 2.
func NewV2(db *pgxpool.Pool, src ImageSource, windowSize int) *WorkerImpl2 {
	return &WorkerImpl2{
		db:                       db,
		imageSource:              src,
		windowSize:               windowSize,
		badDigestsCache:          ttlcache.New(badImageCooldown, 2*badImageCooldown),
		metricsCalculatedCounter: metrics2.GetCounter("diffcalculator_metricscalculated"),
		inputDigestsSummary:      metrics2.GetFloat64SummaryMetric("diffcalculator_inputdigests"),
		digestsOfInterestSummary: metrics2.GetFloat64SummaryMetric("diffcalculator_digestsofinterest"),
	}
}

// CalculateDiffs calculates the diffs for the given grouping. It either computes all of the diffs
// if there are only "a few" digests, otherwise it computes a subset of them, taking into account
// recency and triage status.
func (w *WorkerImpl2) CalculateDiffs(ctx context.Context, grouping paramtools.Params, addLeft, addRight []types.Digest) error {
	ctx, span := trace.StartSpan(ctx, "worker2_CalculateDiffs")
	if span.IsRecordingEvents() {
		addMetadata(span, grouping, len(addLeft), len(addRight))
	}
	defer span.End()
	startingTile, err := w.getStartingTile(ctx)
	if err != nil {
		return skerr.Wrapf(err, "get starting tile")
	}
	allDigests, err := w.getAllExisting(ctx, startingTile, grouping)
	if err != nil {
		return skerr.Wrap(err)
	}

	total := len(allDigests) + len(addLeft) + len(addRight)
	w.inputDigestsSummary.Observe(float64(total))
	inputDigests := convertToDigestBytes(append(addLeft, addRight...))
	if total > computeTotalGridCutoff {
		// If there are too many digests, we perform a somewhat expensive operation of looking at
		// the digests produced by all traces to find a smaller subset of images that we should
		// use to compute diffs for. We don't want to do this all the time because we expect
		// a small percentage of groupings (i.e. tests) to have many digests.
		return skerr.Wrap(w.calculateDiffSubset(ctx, grouping, inputDigests, startingTile))
	}
	allDigests = addDigests(allDigests, addLeft)
	allDigests = addDigests(allDigests, addRight)
	return skerr.Wrap(w.calculateAllDiffs(ctx, allDigests))
}

// convertToDigestBytes converts a slice of hex-encoded digests to bytes (the native type in the
// SQL DB. If any are invalid, they are ignored.
func convertToDigestBytes(digests []types.Digest) []schema.DigestBytes {
	rv := make([]schema.DigestBytes, 0, len(digests))
	for _, d := range digests {
		b, err := sql.DigestToBytes(d)
		if err != nil {
			sklog.Warningf("Invalid digest seen %q: %s", d, err)
			continue
		}
		rv = append(rv, b)
	}
	return rv
}

// addDigests adds the given hex-encoded digests to the slice of bytes.
func addDigests(digests []schema.DigestBytes, additional []types.Digest) []schema.DigestBytes {
	if len(additional) == 0 {
		return digests
	}
	for _, d := range additional {
		b, err := sql.DigestToBytes(d)
		if err != nil {
			sklog.Warningf("Invalid digest seen %q: %s", d, err)
			continue
		}
		digests = append(digests, b)
	}
	return digests
}

// getStartingTile returns the commit ID which is the beginning of the tile of interest (so we
// get enough data to do our comparisons).
func (w *WorkerImpl2) getStartingTile(ctx context.Context) (schema.TileID, error) {
	if w.windowSize <= 0 {
		return 0, nil
	}
	const statement = `WITH
RecentCommits AS (
	SELECT tile_id, commit_id FROM CommitsWithData
	AS OF SYSTEM TIME '-0.1s'
	ORDER BY commit_id DESC LIMIT $1
)
SELECT tile_id FROM RecentCommits
AS OF SYSTEM TIME '-0.1s'
ORDER BY commit_id ASC LIMIT 1`
	row := w.db.QueryRow(ctx, statement, w.windowSize)
	var lc pgtype.Int4
	if err := row.Scan(&lc); err != nil {
		if err == pgx.ErrNoRows {
			return 0, nil // not enough commits seen, so start at tile 0.
		}
		return 0, skerr.Wrapf(err, "getting latest commit")
	}
	if lc.Status == pgtype.Null {
		// There are no commits with data, so start at tile 0.
		return 0, nil
	}
	return schema.TileID(lc.Int), nil
}

// getAllExisting returns the unique digests that were seen on the primary branch for a given
// grouping starting at the given commit.
func (w *WorkerImpl2) getAllExisting(ctx context.Context, beginTileStart schema.TileID, grouping paramtools.Params) ([]schema.DigestBytes, error) {
	if w.windowSize <= 0 {
		return nil, nil
	}
	ctx, span := trace.StartSpan(ctx, "getAllExisting")
	defer span.End()
	const statement = `
WITH
TracesMatchingGrouping AS (
  SELECT trace_id FROM Traces WHERE grouping_id = $1
)
SELECT DISTINCT digest FROM TiledTraceDigests
JOIN TracesMatchingGrouping on TiledTraceDigests.trace_id = TracesMatchingGrouping.trace_id
WHERE TiledTraceDigests.tile_id >= $2`

	_, groupingID := sql.SerializeMap(grouping)
	rows, err := w.db.Query(ctx, statement, groupingID, beginTileStart)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching digests")
	}
	defer rows.Close()
	var rv []schema.DigestBytes
	for rows.Next() {
		var d schema.DigestBytes
		if err := rows.Scan(&d); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, d)
	}
	return rv, nil
}

// calculateAllDiffs calculates all diffs between each digest in the slice and all other digests.
// If there are duplicates in the given slice, they will be removed and not double-calculated.
func (w *WorkerImpl2) calculateAllDiffs(ctx context.Context, digests []schema.DigestBytes) error {
	if len(digests) == 0 {
		return nil
	}
	ctx, span := trace.StartSpan(ctx, "calculateAllDiffs")
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("num_digests", int64(len(digests))))
	missingWork, err := w.getMissingDiffs(ctx, digests)
	if err != nil {
		return skerr.Wrap(err)
	}
	if len(missingWork) == 0 {
		sklog.Infof("All diffs are already calculated")
		return nil
	}
	span.AddAttributes(trace.Int64Attribute("num_diffs", int64(len(missingWork))))
	if err := w.computeDiffsInParallel(ctx, missingWork); err != nil {
		return skerr.Wrapf(err, "calculating %d diffs for %d digests", len(missingWork), len(digests))
	}
	return nil
}

// getMissingDiffs creates a half-square of diffs, where each digest is compared to every other
// digest. Then, it returns all those digestPairs of diffs that have not already been calculated.
func (w *WorkerImpl2) getMissingDiffs(ctx context.Context, digests []schema.DigestBytes) ([]digestPair, error) {
	ctx, span := trace.StartSpan(ctx, "getMissingDiffs")
	span.AddAttributes(trace.Int64Attribute("num_digests", int64(len(digests))))
	defer span.End()

	possibleWork := map[digestPair]bool{}
	for i := range digests {
		left := types.Digest(hex.EncodeToString(digests[i]))
		for j := i + 1; j < len(digests); j++ {
			right := types.Digest(hex.EncodeToString(digests[j]))
			if left == right {
				continue
			}
			dp := newDigestPair(left, right)
			possibleWork[dp] = true
		}
	}
	span.AddAttributes(trace.Int64Attribute("possible_work_size", int64(len(possibleWork))))

	const statement = `SELECT DISTINCT encode(left_digest, 'hex'), encode(right_digest, 'hex')
FROM DiffMetrics AS OF SYSTEM TIME '-0.1s'
WHERE left_digest = ANY($1) AND right_digest = ANY($1)`
	rows, err := w.db.Query(ctx, statement, digests)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	for rows.Next() {
		if err := ctx.Err(); err != nil {
			return nil, skerr.Wrapf(err, "context error")
		}
		var left, right types.Digest
		if err := rows.Scan(&left, &right); err != nil {
			return nil, skerr.Wrap(err)
		}
		dp := newDigestPair(left, right)
		possibleWork[dp] = false
	}

	var toCalculate []digestPair
	for dp, needsComputing := range possibleWork {
		if needsComputing {
			toCalculate = append(toCalculate, dp)
		}
	}
	return toCalculate, nil
}

func (w *WorkerImpl2) calculateDiffSubset(ctx context.Context, grouping paramtools.Params, digests []schema.DigestBytes, startingTile schema.TileID) error {
	ctx, span := trace.StartSpan(ctx, "calculateDiffSubset")
	defer span.End()
	span.AddAttributes(trace.Int64Attribute("starting_digests", int64(len(digests))))

	allTriaged, err := w.getTriagedDigests(ctx, grouping, startingTile)
	if err != nil {
		return skerr.Wrap(err)
	}
	span.AddAttributes(trace.Int64Attribute("triaged_digests", int64(len(allTriaged))))

	commonAndRecentDigests, err := w.getCommonAndRecentDigests(ctx, grouping)
	if err != nil {
		return skerr.Wrap(err)
	}
	span.AddAttributes(trace.Int64Attribute("common_and_recent_digests", int64(len(commonAndRecentDigests))))

	digests = append(digests, allTriaged...)
	digests = append(digests, commonAndRecentDigests...)
	// note: The digests slice could have duplicates, but that won't inflate the work we need to
	// compute.
	w.digestsOfInterestSummary.Observe(float64(len(digests)))
	sklog.Infof("Got around %d digests of interest for grouping %#v", len(digests), grouping)
	return skerr.Wrapf(w.calculateAllDiffs(ctx, digests), "calculating diffs for %d digests in grouping %#v", len(digests), grouping)
}

func (w *WorkerImpl2) computeDiffsInParallel(ctx context.Context, work []digestPair) error {
	ctx, span := trace.StartSpan(ctx, "computeDiffsInParallel")
	span.AddAttributes(trace.Int64Attribute("num_diffs", int64(len(work))))
	defer span.End()
	sklog.Infof("Calculating %d new diffs", len(work))
	imgCache, err := lru.New(decodedImageCacheSize)
	if err != nil {
		return skerr.Wrap(err)
	}
	ctx = addImgCache(ctx, imgCache)
	defer func() {
		imgCache.Purge() // Make it easier to GC anything left in the cache.
	}()

	chunkSize := len(work)/diffingRoutines + 1 // add 1 to avoid integer division to 0
	err = util.ChunkIterParallel(ctx, len(work), chunkSize, func(ctx context.Context, startIdx int, endIdx int) error {
		batch := work[startIdx:endIdx]
		metricsBuffer := make([]schema.DiffMetricRow, 0, reportingBatchSize)
		for _, pair := range batch {
			if err := ctx.Err(); err != nil {
				return skerr.Wrapf(err, "context error")
			}
			// If either image is known to be bad, skip this work
			_, bad1 := w.badDigestsCache.Get(string(pair.left))
			_, bad2 := w.badDigestsCache.Get(string(pair.right))
			if bad1 || bad2 {
				continue
			}

			nm, iErr := w.diff(ctx, pair.left, pair.right)
			// If there is an error diffing, it is because we couldn't download or decode
			// one of the images. If so, we skip that entry and report it, before moving on.
			if iErr != nil {
				if err := ctx.Err(); err != nil {
					return skerr.Wrap(err)
				}
				// Add the bad image to the ttlcache so we skip it for a short while.
				w.badDigestsCache.Set(string(iErr.digest), true, ttlcache.DefaultExpiration)
				if err := w.reportProblemImage(ctx, iErr); err != nil {
					return skerr.Wrap(err)
				}
				continue
			}
			metricsBuffer = append(metricsBuffer, nm)
			if len(metricsBuffer) > reportingBatchSize {
				if err := w.writeMetrics(ctx, metricsBuffer); err != nil {
					return skerr.Wrap(err)
				}
				w.metricsCalculatedCounter.Inc(int64(len(metricsBuffer)))
				metricsBuffer = metricsBuffer[:0] // reset buffer
			}
		}
		if err := w.writeMetrics(ctx, metricsBuffer); err != nil {
			return skerr.Wrap(err)
		}
		w.metricsCalculatedCounter.Inc(int64(len(metricsBuffer)))
		return nil
	})
	return skerr.Wrap(err)
}

// diff calculates the difference between the two images with the provided digests and returns
// it in a format that can be inserted into the SQL database. If there is an error downloading
// or decoding a digest, an error is returned along with the problematic digest.
func (w *WorkerImpl2) diff(ctx context.Context, left, right types.Digest) (schema.DiffMetricRow, *imgError) {
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
		Timestamp:         now.Now(ctx),
	}, nil
}

// getImage retrieves and decodes the given image. If the image is cached, this function will
// return the cached version. We choose to cache the decoded image (and not just the downloaded
// image) because the decoding tends to take 3-5x longer than downloading.
func (w *WorkerImpl2) getDecodedImage(ctx context.Context, digest types.Digest) (*image.NRGBA, error) {
	ctx, span := trace.StartSpan(ctx, "getDecodedImage")
	defer span.End()
	cache := getImgCache(ctx)
	if cache != nil {
		if cachedImg, ok := cache.Get(string(digest)); ok {
			return cachedImg.(*image.NRGBA), nil
		}
	}
	b, err := w.imageSource.GetImage(ctx, digest)
	if err != nil {
		return nil, skerr.Wrapf(err, "getting image with digest %s", digest)
	}
	img, err := decode(ctx, b)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	// In memory, the image takes up 4 bytes per pixel.
	s := img.Bounds().Size()
	sizeInBytes := int64(s.X * s.Y * 4)
	span.AddAttributes(trace.Int64Attribute("size_in_bytes", sizeInBytes))
	if cache != nil {
		cache.Add(string(digest), img)
	}
	return img, nil
}

// writeMetrics writes two copies of the provided metrics (one for left-right and one for
// right-left) to the SQL database.
func (w *WorkerImpl2) writeMetrics(ctx context.Context, metrics []schema.DiffMetricRow) error {
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
func (w *WorkerImpl2) reportProblemImage(ctx context.Context, imgErr *imgError) error {
	ctx, span := trace.StartSpan(ctx, "reportProblemImage")
	defer span.End()
	sklog.Warningf("Reporting problem with image %s: %s", imgErr.digest, imgErr.err)
	const statement = `
INSERT INTO ProblemImages (digest, num_errors, latest_error, error_ts)
VALUES ($1, $2, $3, $4)
ON CONFLICT (digest)
DO UPDATE SET (num_errors, latest_error, error_ts) =
(ProblemImages.num_errors + 1, $3, $4)`

	_, err := w.db.Exec(ctx, statement, imgErr.digest, 1, imgErr.err.Error(), now.Now(ctx))
	if err != nil {
		return skerr.Wrapf(err, "writing to ProblemImages")
	}
	return nil
}

// getTriagedDigests returns all triaged digests (positive and negative) for the given grouping
// seen in the given tile or later.
func (w *WorkerImpl2) getTriagedDigests(ctx context.Context, grouping paramtools.Params, startingTile schema.TileID) ([]schema.DigestBytes, error) {
	ctx, span := trace.StartSpan(ctx, "getTriagedDigests")
	defer span.End()
	const statement = `WITH
TracesMatchingGrouping AS (
  SELECT trace_id FROM Traces WHERE grouping_id = $1
),
RecentDigestsMatchingGrouping AS (
	SELECT DISTINCT digest FROM TiledTraceDigests
	JOIN TracesMatchingGrouping on TiledTraceDigests.trace_id = TracesMatchingGrouping.trace_id
	WHERE TiledTraceDigests.tile_id >= $2
)
SELECT Expectations.digest FROM Expectations
JOIN RecentDigestsMatchingGrouping ON RecentDigestsMatchingGrouping.digest = Expectations.digest
	AND Expectations.grouping_id = $1
WHERE Expectations.label = 'p' OR Expectations.label = 'n'`

	_, groupingID := sql.SerializeMap(grouping)
	rows, err := w.db.Query(ctx, statement, groupingID, startingTile)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var rv []schema.DigestBytes
	for rows.Next() {
		var b schema.DigestBytes
		if err := rows.Scan(&b); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, b)
	}
	return rv, nil
}

// getCommonAndRecentDigests returns a set of digests seen on traces for the given grouping that
// are either commonly seen in the current window or recently seen. This approach prevents us from
// calculating too many diffs that "aren't worth it", commonly when there are several flaky traces
// in a grouping.
func (w *WorkerImpl2) getCommonAndRecentDigests(ctx context.Context, grouping paramtools.Params) ([]schema.DigestBytes, error) {
	ctx, span := trace.StartSpan(ctx, "getCommonAndRecentDigests")
	defer span.End()

	row := w.db.QueryRow(ctx, `WITH
RecentCommits AS (
	SELECT commit_id FROM CommitsWithData
	ORDER BY commit_id DESC LIMIT $1
)
SELECT commit_id FROM RecentCommits
ORDER BY commit_id ASC LIMIT 1`, w.windowSize)
	var firstCommitID string
	if err := row.Scan(&firstCommitID); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, skerr.Wrap(err)
	}

	const statement = `WITH
TracesOfInterest AS (
  SELECT trace_id FROM Traces WHERE grouping_id = $1
),
DigestsCountsAndMostRecent AS (
  SELECT digest, count(*) as occurrences, max(commit_id) as recent FROM TraceValues
  JOIN TracesOfInterest ON TraceValues.trace_id = TracesOfInterest.trace_id
  WHERE commit_id >= $2
  GROUP BY digest
),
MostCommon AS (
    -- The number 50 here was chosen after some experimentation with the Skia instance.
    SELECT digest FROM DigestsCountsAndMostRecent ORDER BY occurrences DESC, recent DESC LIMIT 50
),
MostRecent AS (
	-- The number 300 here was chosen after some experimentation with the Skia instance. At the
	-- time of writing, a typical grouping had ~1000 traces associated with it. A small percentage
	-- of these traces might be flaky. 300 seems like a good balance between making sure we can
	-- compute some digests for each trace, without losing to the quadratic cost of computing too
	-- many digests.
    SELECT digest FROM DigestsCountsAndMostRecent ORDER BY recent DESC, occurrences DESC LIMIT 300
)
-- The UNION operator will remove any duplicate digests
SELECT * FROM MostCommon
UNION
SELECT * FROM MostRecent`

	_, groupingID := sql.SerializeMap(grouping)
	rows, err := w.db.Query(ctx, statement, groupingID, firstCommitID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	var rv []schema.DigestBytes
	for rows.Next() {
		var b schema.DigestBytes
		if err := rows.Scan(&b); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, b)
	}
	return rv, nil
}

// Make sure WorkerImpl fulfills the diff.Calculator interface.
var _ diff.Calculator = (*WorkerImpl2)(nil)
