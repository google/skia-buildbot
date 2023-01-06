package worker

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"time"

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
	"go.skia.org/infra/go/sql/sqlutil"
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
	// This number was chosen based on experimentation with the Skia instance (see also
	// getCommonAndRecentDigests
	computeTotalGridCutoff = 300
	// Downloading and decoding images appears to be a bottleneck for diffing. We spin up a small
	// cache for each diff message to help retain those images while we calculate the diffs.
	// This number was chosen to be above the number of digests returned by
	// getCommonAndRecentDigests, because downloading and decoding images is the most
	// computationally expensive of the whole process.
	decodedImageCacheSize = 1000
	// In an effort to prevent spamming the ProblemImages database, we skip known bad images for
	// a period of time. This time is controlled by badImageCooldown and a TTL cache.
	badImageCooldown = time.Minute

	diffingRoutines = 4

	// This batch size corresponds to tens of seconds worth of computation. If we are
	// interrupted, we hope not to lose more than this amount of work.
	reportingBatchSize = 25
)

// ImageSource is an abstraction around a way to load the images. If images are stored in GCS, or
// on a file system or wherever, they should be provided by this mechanism.
type ImageSource interface {
	// GetImage returns the raw bytes of an image with the corresponding Digest.
	GetImage(ctx context.Context, digest types.Digest) ([]byte, error)
}

type WorkerImpl struct {
	db              *pgxpool.Pool
	imageSource     ImageSource
	badDigestsCache *ttlcache.Cache
	windowSize      int

	inputDigestsSummary      metrics2.Float64SummaryMetric
	digestsOfInterestSummary metrics2.Float64SummaryMetric
	metricsCalculatedCounter metrics2.Counter
}

// New returns a diff worker which uses the provided ImageSource.
func New(db *pgxpool.Pool, src ImageSource, windowSize int) *WorkerImpl {
	return &WorkerImpl{
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
func (w *WorkerImpl) CalculateDiffs(ctx context.Context, grouping paramtools.Params, additional []types.Digest) error {
	ctx, span := trace.StartSpan(ctx, "worker2_CalculateDiffs")
	if span.IsRecordingEvents() {
		addMetadata(span, grouping, len(additional))
	}
	defer span.End()
	startingTile, endingTile, err := w.getTileBounds(ctx)
	if err != nil {
		return skerr.Wrapf(err, "get starting tile")
	}
	allDigests, err := w.getAllExisting(ctx, startingTile, endingTile, grouping)
	if err != nil {
		return skerr.Wrap(err)
	}

	total := len(allDigests) + len(additional)
	w.inputDigestsSummary.Observe(float64(total))
	inputDigests := convertToDigestBytes(additional)
	if total > computeTotalGridCutoff {
		// If there are too many digests, we perform a somewhat expensive operation of looking at
		// the digests produced by all traces to find a smaller subset of images that we should
		// use to compute diffs for. We don't want to do this all the time because we expect
		// a small percentage of groupings (i.e. tests) to have many digests.
		return skerr.Wrap(w.calculateDiffSubset(ctx, grouping, inputDigests, startingTile))
	}
	allDigests = append(allDigests, inputDigests...)
	return skerr.Wrap(w.calculateAllDiffs(ctx, allDigests))
}

// addMetadata adds some attributes to the span so we can tell how much work it was supposed to
// be doing when we are looking at the traces and the performance.
func addMetadata(span *trace.Span, grouping paramtools.Params, leftDigestCount int) {
	groupingStr, _ := json.Marshal(grouping)
	span.AddAttributes(
		trace.StringAttribute("grouping", string(groupingStr)),
		trace.Int64Attribute("additional_digests", int64(leftDigestCount)))
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

// getTileBounds returns the tile id corresponding to the commit of the beginning of the window,
// as well as the most recent tile id (so we get enough data to do our comparisons).
func (w *WorkerImpl) getTileBounds(ctx context.Context) (schema.TileID, schema.TileID, error) {
	if w.windowSize <= 0 {
		return 0, 0, nil
	}
	const statement = `WITH
RecentCommits AS (
	SELECT tile_id, commit_id FROM CommitsWithData
	AS OF SYSTEM TIME '-0.1s'
	ORDER BY commit_id DESC LIMIT $1
)
SELECT MIN(tile_id), MAX(tile_id) FROM RecentCommits
AS OF SYSTEM TIME '-0.1s'
`
	row := w.db.QueryRow(ctx, statement, w.windowSize)
	var lc pgtype.Int4
	var mc pgtype.Int4
	if err := row.Scan(&lc, &mc); err != nil {
		if err == pgx.ErrNoRows {
			return 0, 0, nil // not enough commits seen, so start at tile 0.
		}
		return 0, 0, skerr.Wrapf(err, "getting latest commit")
	}
	if lc.Status == pgtype.Null || mc.Status == pgtype.Null {
		// There are no commits with data, so start at tile 0.
		return 0, 0, nil
	}
	return schema.TileID(lc.Int), schema.TileID(mc.Int), nil
}

// getAllExisting returns the unique digests that were seen on the primary branch for a given
// grouping starting at the given commit.
func (w *WorkerImpl) getAllExisting(ctx context.Context, beginTile, endTile schema.TileID, grouping paramtools.Params) ([]schema.DigestBytes, error) {
	if w.windowSize <= 0 {
		return nil, nil
	}
	ctx, span := trace.StartSpan(ctx, "getAllExisting")
	defer span.End()

	_, groupingID := sql.SerializeMap(grouping)
	tracesForGroup, err := w.getTracesForGroup(ctx, groupingID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	tilesInRange := make([]schema.TileID, 0, endTile-beginTile+1)
	for i := beginTile; i <= endTile; i++ {
		tilesInRange = append(tilesInRange, i)
	}
	// We tried doing "fetch traces for group" and "fetch TiledTraceDigests" all in one query.
	// However, that took a long while in the large Skia repo. The reason being that the index
	// was not used effectively. Our index is on tile_id + trace_id, however CockroachDB was
	// having to effectively fetch all rows that came after the starting tile id, even though it
	// only needed < 0.1% of them (assuming 1000 groupings). The to do this smarter is to specify
	// exactly what tiles we need (not just "greater than this tile") and exactly what traces we
	// need. This lets CockroachDB only fetch the portions of the index it needs to. We saw this
	// be 10x more effective than the previous query with a join.
	const statement = `
SELECT DISTINCT digest FROM TiledTraceDigests
AS OF SYSTEM TIME '-0.1s'
WHERE tile_id = ANY($1) AND trace_id = ANY($2)`

	rows, err := w.db.Query(ctx, statement, tilesInRange, tracesForGroup)
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

// getTracesForGroup returns all the traces that are a part of the specified grouping.
func (w *WorkerImpl) getTracesForGroup(ctx context.Context, id schema.GroupingID) ([]schema.TraceID, error) {
	ctx, span := trace.StartSpan(ctx, "getTracesForGroup")
	defer span.End()
	const statement = `SELECT trace_id FROM Traces
AS OF SYSTEM TIME '-0.1s'
WHERE grouping_id = $1`
	rows, err := w.db.Query(ctx, statement, id)
	if err != nil {
		return nil, skerr.Wrapf(err, "fetching trace ids")
	}
	defer rows.Close()
	var rv []schema.TraceID
	for rows.Next() {
		var t schema.TraceID
		if err := rows.Scan(&t); err != nil {
			return nil, skerr.Wrap(err)
		}
		rv = append(rv, t)
	}
	return rv, nil
}

// calculateAllDiffs calculates all diffs between each digest in the slice and all other digests.
// If there are duplicates in the given slice, they will be removed and not double-calculated.
func (w *WorkerImpl) calculateAllDiffs(ctx context.Context, digests []schema.DigestBytes) error {
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
func (w *WorkerImpl) getMissingDiffs(ctx context.Context, digests []schema.DigestBytes) ([]digestPair, error) {
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

func (w *WorkerImpl) calculateDiffSubset(ctx context.Context, grouping paramtools.Params, digests []schema.DigestBytes, startingTile schema.TileID) error {
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

func (w *WorkerImpl) computeDiffsInParallel(ctx context.Context, work []digestPair) error {
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
		Timestamp:         now.Now(ctx),
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

// getImage retrieves and decodes the given image. If the image is cached, this function will
// return the cached version. We choose to cache the decoded image (and not just the downloaded
// image) because the decoding tends to take 3-5x longer than downloading.
func (w *WorkerImpl) getDecodedImage(ctx context.Context, digest types.Digest) (*image.NRGBA, error) {
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
	vp := sqlutil.ValuesPlaceholders(valuesPerRow, count)
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

	_, err := w.db.Exec(ctx, statement, imgErr.digest, 1, imgErr.err.Error(), now.Now(ctx))
	if err != nil {
		return skerr.Wrapf(err, "writing to ProblemImages")
	}
	return nil
}

// getTriagedDigests returns all triaged digests (positive and negative) for the given grouping
// seen in the given tile or later.
func (w *WorkerImpl) getTriagedDigests(ctx context.Context, grouping paramtools.Params, startingTile schema.TileID) ([]schema.DigestBytes, error) {
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
func (w *WorkerImpl) getCommonAndRecentDigests(ctx context.Context, grouping paramtools.Params) ([]schema.DigestBytes, error) {
	ctx, span := trace.StartSpan(ctx, "getCommonAndRecentDigests")
	defer span.End()

	// Find the first commit in the target range of commits.
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

	// Find all the traces for the target grouping.
	traceIDs, err := w.findTraceIDsForGrouping(ctx, grouping)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	// We used to join the Traces and TraceValues tables, but that turned out to be slow. Filtering
	// TraceValues using a list of trace IDs can be faster when the list of trace IDs is large.
	// For example, lovisolo@ observed that for a grouping with ~1000 traces, the below query takes
	// ~40s when joining the two tables, or ~6s when using "WHERE trace_id IN (id1, id2, ...)".
	traceIDPlaceholders := sqlutil.ValuesPlaceholders(1 /* =valuesPerRow */, len(traceIDs))

	statement := `WITH
DigestsCountsAndMostRecent AS (
  SELECT digest, count(*) as occurrences, max(commit_id) as recent FROM TraceValues
  WHERE trace_id IN (` + traceIDPlaceholders + `) AND commit_id >= ` + fmt.Sprintf("$%d", len(traceIDs)+1) + `
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

	// Build a list of arguments with the IDs of all traces of interest and the firstCommitID.
	args := make([]interface{}, 0, len(traceIDs)+1)
	for _, traceID := range traceIDs {
		args = append(args, traceID)
	}
	args = append(args, firstCommitID)

	// Execute the statement and return the result.
	rows, err := w.db.Query(ctx, statement, args...)
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

// findTraceIDsForGrouping returns the trace IDs corresponding to the given grouping.
func (w *WorkerImpl) findTraceIDsForGrouping(ctx context.Context, grouping paramtools.Params) ([]schema.TraceID, error) {
	ctx, span := trace.StartSpan(ctx, "findTraceIDsForGrouping")
	defer span.End()

	_, groupingID := sql.SerializeMap(grouping)
	rows, err := w.db.Query(ctx, "SELECT trace_id FROM Traces WHERE grouping_id = $1", groupingID)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()

	var traceIDs []schema.TraceID
	for rows.Next() {
		var tid schema.TraceID
		if err := rows.Scan(&tid); err != nil {
			return nil, skerr.Wrap(err)
		}
		traceIDs = append(traceIDs, tid)
	}
	return traceIDs, nil
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

type imgError struct {
	err    error
	digest types.Digest
}

type contextType string

const imgCacheContextKey contextType = "imgCache"

// addImgCache adds a cache of decoded images to the context, so we can use it in leaf
// functions more easily.
func addImgCache(ctx context.Context, cache *lru.Cache) context.Context {
	return context.WithValue(ctx, imgCacheContextKey, cache)
}

func getImgCache(ctx context.Context) *lru.Cache {
	c, ok := ctx.Value(imgCacheContextKey).(*lru.Cache)
	if !ok {
		return nil
	}
	return c
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
