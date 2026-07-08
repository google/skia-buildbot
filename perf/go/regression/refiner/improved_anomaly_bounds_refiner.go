package refiner

import (
	"context"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/anomalies"
	"go.skia.org/infra/perf/go/git"
	"go.skia.org/infra/perf/go/regression"
	"go.skia.org/infra/perf/go/stepfit"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/sync/errgroup"
)

// ImprovedAnomalyBoundsRefiner implements regression.RegressionRefiner.
// It runs the standard AnomalyBoundsRefiner first, and then applies
// additional refinement logic based on previous regressions found in the database
// and loading raw data from the database.
type ImprovedAnomalyBoundsRefiner struct {
	base            *AnomalyBoundsRefiner
	anomalyStore    anomalies.Store
	store           regression.Store
	traceStore      tracestore.TraceStore
	perfGit         git.Git
	stdDevThreshold float32
	dryRun          bool // No longer used, TODO(mordeckimarcin) cleanup.
}

// NewImprovedAnomalyBoundsRefiner returns a new instance of ImprovedAnomalyBoundsRefiner.
func NewImprovedAnomalyBoundsRefiner(anomalyStore anomalies.Store, store regression.Store, traceStore tracestore.TraceStore, perfGit git.Git, stdDevThreshold float32, dryRun bool) *ImprovedAnomalyBoundsRefiner {
	return &ImprovedAnomalyBoundsRefiner{
		base:            &AnomalyBoundsRefiner{stdDevThreshold: stdDevThreshold},
		anomalyStore:    anomalyStore,
		store:           store,
		traceStore:      traceStore,
		perfGit:         perfGit,
		stdDevThreshold: stdDevThreshold,
		dryRun:          dryRun,
	}
}

// Process implements the regression.RegressionRefiner interface.
// It provides additional filtering by looking at the previous regression (previous change point).
// This allows us to get much more context without increasing the radius, enabling the application
// to more effectively separate noise from real regressions.
func (r *ImprovedAnomalyBoundsRefiner) Process(ctx context.Context, cfg *alerts.Alert, responses []*regression.RegressionDetectionResponse) ([]*regression.ConfirmedRegression, error) {
	// Run the base AnomalyBoundsRefiner logic.
	start := time.Now()
	responsesLen := len(responses)
	sklog.Debugf("starting base refiner logic for %d responses", responsesLen)
	confirmed, err := r.base.Process(ctx, cfg, responses)
	if err != nil {
		return nil, err
	}
	sklog.Debugf("base refiner logic for %d responses done in %s", responsesLen, time.Since(start))
	start = time.Now()

	// The Const algorithm ignores the baseline, so the improved refiner logic
	// (which relies on a historical baseline) is not applicable.
	if cfg.Step == types.Const {
		return confirmed, nil
	}

	byTrace := groupRegressionsByTrace(confirmed)

	traceNames := make([]string, 0, len(byTrace))
	for name := range byTrace {
		traceNames = append(traceNames, name)
	}
	sort.Strings(traceNames)
	sklog.Debugf("starting batched regressions before - %d resp", responsesLen)

	sklog.Debugf("regressions grouped by trace in %s", time.Since(start))
	start = time.Now()

	batchPrev, err := r.getBatchRegressionsBefore(ctx, confirmed, cfg.SubscriptionName)
	if err != nil {
		return nil, skerr.Wrapf(err, "[ImprovedAnomalyBoundsRefiner] Failed to get batch regressions before")
	}
	sklog.Debugf("batched regressions queries - %d resp in %s", responsesLen, time.Since(start))
	start = time.Now()

	rangesToFetch := r.calculateRangesToFetch(byTrace, batchPrev)

	fetchedTraces, fetchedCommits, err := r.traceStore.ReadTracesForCommitRanges(ctx, rangesToFetch)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to batch fetch trace data")
	}

	pData := &prefetchedData{
		traces:  fetchedTraces,
		commits: fetchedCommits,
	}
	sklog.Debugf("batch fetched trace data in %s", time.Since(start))
	start = time.Now()

	var mu sync.Mutex
	var refined []*regression.ConfirmedRegression

	g, ctx := errgroup.WithContext(ctx)
	groupLimit := 20
	g.SetLimit(groupLimit)

	for _, traceName := range traceNames {
		traceName := traceName
		group := byTrace[traceName]
		g.Go(func() error {
			localRefined := r.processTraceGroup(group, cfg, batchPrev, pData)
			if len(localRefined) > 0 {
				mu.Lock()
				refined = append(refined, localRefined...)
				mu.Unlock()
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, skerr.Wrapf(err, "error processing trace group")
	}
	sklog.Debugf("improved refinement done for %d responses - got %d refined in %s", responsesLen, len(refined), time.Since(start))

	return refined, nil
}

// getBatchRegressionsBefore retrieves historical database regressions for all confirmed candidates.
func (r *ImprovedAnomalyBoundsRefiner) getBatchRegressionsBefore(ctx context.Context, confirmed []*regression.ConfirmedRegression, subscriptionName string) (map[string]map[types.CommitNumber]*regression.Regression, error) {
	if len(confirmed) == 0 {
		return map[string]map[types.CommitNumber]*regression.Regression{}, nil
	}
	var traceNamesBatch []string
	var commitsBatch []types.CommitNumber
	for _, cr := range confirmed {
		traceNamesBatch = append(traceNamesBatch, cr.Summary.Clusters[0].Keys[0])
		commitsBatch = append(commitsBatch, cr.DisplayCommitNumber)
	}
	return r.store.GetBatchRegressionsBefore(ctx, traceNamesBatch, commitsBatch, subscriptionName)
}

// calculateRangesToFetch calculates the commit ranges of trace data to pre-fetch
// for refining a batch of candidate regressions.
//
// For each trace, it determines a single bounding range [begin, end] to fetch:
//   - 'begin': The commit of the previous regression (from the batchPrev database),
//     or the oldest regression in the current batch if no previous database
//     regression exists and there are multiple regressions in the batch.
//   - 'end': The commit immediately preceding the newest regression in the batch.
//
// Traces with only one candidate regression and no previous database regression
// are skipped, as they cannot be refined without a historical baseline.
func (r *ImprovedAnomalyBoundsRefiner) calculateRangesToFetch(byTrace map[string][]*regression.ConfirmedRegression, batchPrev map[string]map[types.CommitNumber]*regression.Regression) map[string]tracestore.TraceRangeRequest {
	rangesToFetch := make(map[string]tracestore.TraceRangeRequest)
	for traceName, group := range byTrace {
		if len(group) == 0 {
			continue
		}
		sort.Slice(group, func(i, j int) bool {
			return group[i].CommitNumber < group[j].CommitNumber
		})

		crOldest := group[0]
		crNewest := group[len(group)-1]

		dbPrevOldest := batchPrev[traceName][crOldest.DisplayCommitNumber]

		var begin types.CommitNumber
		if dbPrevOldest != nil {
			begin = dbPrevOldest.CommitNumber
		} else if len(group) > 1 {
			begin = crOldest.CommitNumber
		} else {
			continue
		}

		end := crNewest.PrevCommitNumber

		if begin <= end {
			rangesToFetch[traceName] = tracestore.TraceRangeRequest{
				BeginCommit: begin,
				EndCommit:   end,
			}
		}
	}
	return rangesToFetch
}

// groupRegressionsByTrace groups a slice of confirmed regressions by their trace name.
func groupRegressionsByTrace(confirmed []*regression.ConfirmedRegression) map[string][]*regression.ConfirmedRegression {
	byTrace := make(map[string][]*regression.ConfirmedRegression)
	for _, cr := range confirmed {
		traceName := cr.Summary.Clusters[0].Keys[0]
		byTrace[traceName] = append(byTrace[traceName], cr)
	}
	return byTrace
}

// processTraceGroup processes a list of regressions for a single trace sequentially,
// tracking and utilizing the previous refined regression in chronological order.
func (r *ImprovedAnomalyBoundsRefiner) processTraceGroup(group []*regression.ConfirmedRegression, cfg *alerts.Alert, batchPrev map[string]map[types.CommitNumber]*regression.Regression, pData *prefetchedData) []*regression.ConfirmedRegression {
	var localRefined []*regression.ConfirmedRegression
	var lastRefined *regression.ConfirmedRegression
	for _, cr := range group {
		newCr := r.applyImprovedLogic(cr, cfg, lastRefined, batchPrev, pData)
		if newCr != nil {
			localRefined = append(localRefined, newCr)
			lastRefined = newCr
		}
	}
	return localRefined
}

func (r *ImprovedAnomalyBoundsRefiner) applyImprovedLogic(cr *regression.ConfirmedRegression, cfg *alerts.Alert, latestRefined *regression.ConfirmedRegression, batchPrev map[string]map[types.CommitNumber]*regression.Regression, pData *prefetchedData) *regression.ConfirmedRegression {
	traceName := cr.Summary.Clusters[0].Keys[0]
	pickOffset := cr.DisplayCommitNumber

	// Find the previous regression to determine the boundary for loading historical data.
	prevInfo := r.findPreviousRegression(traceName, pickOffset, latestRefined, batchPrev)
	if prevInfo == nil {
		sklog.Infof("[ImprovedAnomalyBoundsRefiner] No previous regression found for trace %s before %d. Keeping original regression.", traceName, pickOffset)
		return cr
	}

	// Check for overlap.
	if prevInfo.CommitNumber >= cr.PrevCommitNumber && prevInfo.PrevCommitNumber <= cr.CommitNumber {
		sklog.Infof("[ImprovedAnomalyBoundsRefiner] Filtering out regression at %d due to overlap with existing %s regression at %d", pickOffset, prevInfo.Source, prevInfo.CommitNumber)
		return nil // Filter out
	}

	// Extract data to beetween regressions.
	leftData, leftCommits, rightData, rightCommits := r.extractData(cr, cfg, prevInfo, pData)
	if leftData == nil {
		return cr
	}

	// Re-verify the regression against the historical baseline (data between previous and current regression) to confirm it is statistically significant.
	rule := cfg.DetectionRule
	if rule == nil {
		rule = stepfit.NewSimpleRule(cfg.Step, cfg.Interesting)
	}

	regressionVal, stepSize, interestingThreshold, isConfirmed := r.calculateStepAndConfirm(leftData, rightData, rule)

	// Filter out if not confirmed.
	if !isConfirmed {
		leftStart := leftCommits[0]
		leftEnd := leftCommits[len(leftCommits)-1]
		rightStart := rightCommits[0]
		rightEnd := rightCommits[len(rightCommits)-1]
		sklog.Infof("[ImprovedAnomalyBoundsRefiner] Filtering out regression for trace %s at offset %d. Failed strict check. RegressionVal: %f, Threshold: %f, Left(mean=%f, stddev=%f, n=%d, range=[%d, %d]), Right(mean=%f, stddev=%f, n=%d, range=[%d, %d]), Pick Range: [%d, %d]",
			traceName, pickOffset, regressionVal, interestingThreshold, vec32.Mean(leftData), vec32.StdDev(leftData, vec32.Mean(leftData)), len(leftData), leftStart, leftEnd, vec32.Mean(rightData), vec32.StdDev(rightData, vec32.Mean(rightData)), len(rightData), rightStart, rightEnd, cr.PrevCommitNumber, cr.CommitNumber)
		return nil
	}

	// Populate metadata to simplify future analysis.
	cr.Message = fmt.Sprintf("%s | Confirmed by ImprovedAnomalyBoundsRefiner with regression value: %f, step size: %f", cr.Message, regressionVal, stepSize)
	sklog.Infof("[ImprovedAnomalyBoundsRefiner] Confirmed regression for trace %s at offset %d. RegressionVal: %f, StepSize: %f", traceName, pickOffset, regressionVal, stepSize)

	if len(cr.Summary.Clusters) > 0 {
		cl := cr.Summary.Clusters[0]
		if cl.Metadata == nil {
			cl.Metadata = map[string]interface{}{}
		}
		cl.Metadata["improved_refiner_left_part"] = leftData
		cl.Metadata["improved_refiner_right_part"] = rightData

		algo := string(cfg.Step)
		if len(cl.StepFit.RuleEvaluations) > 0 {
			algo = cl.StepFit.RuleEvaluations[0].AlgoName
		}
		cl.Metadata["improved_refiner_algo"] = algo
		cl.Metadata["improved_refiner_threshold"] = interestingThreshold
	}

	return cr
}

type previousRegressionInfo struct {
	CommitNumber     types.CommitNumber
	PrevCommitNumber types.CommitNumber
	Source           string
}

// findPreviousRegression looks for the most recent regression on the same trace.
// It checks the pre-loaded batch map first, and compares it with the latest
// regression found in the current processing batch, returning the newer one.
func (r *ImprovedAnomalyBoundsRefiner) findPreviousRegression(traceName string, pickOffset types.CommitNumber, latestRefined *regression.ConfirmedRegression, batchPrev map[string]map[types.CommitNumber]*regression.Regression) *previousRegressionInfo {
	var dbRegression *regression.Regression

	if batchPrev != nil {
		if tMap, ok := batchPrev[traceName]; ok {
			dbRegression = tMap[pickOffset]
		}
	}

	var lastCommit types.CommitNumber
	var lastPrevCommit types.CommitNumber
	found := false
	source := "DB"

	if dbRegression != nil {
		lastCommit = dbRegression.CommitNumber
		lastPrevCommit = dbRegression.PrevCommitNumber
		found = true
	}

	if latestRefined != nil {
		if !found || latestRefined.CommitNumber > lastCommit {
			lastCommit = latestRefined.CommitNumber
			lastPrevCommit = latestRefined.PrevCommitNumber
			found = true
			source = "in-memory"
			sklog.Infof("[ImprovedAnomalyBoundsRefiner] Using in-memory previous regression at %d instead of DB.", latestRefined.CommitNumber)
		}
	}

	if !found {
		return nil
	}

	return &previousRegressionInfo{
		CommitNumber:     lastCommit,
		PrevCommitNumber: lastPrevCommit,
		Source:           source,
	}
}

// extractRightData extracts the right side data points from the centroid of the
// right-most regression in the group.
func extractRightData(cr *regression.ConfirmedRegression) ([]float32, []types.CommitNumber) {
	tpIndex := cr.RightMostSummary.Clusters[0].StepFit.TurningPoint
	var cleanRightData []float32
	var rightCommits []types.CommitNumber
	end := len(cr.RightMostSummary.Clusters[0].Centroid)
	for i := tpIndex; i < end; i++ {
		v := cr.RightMostSummary.Clusters[0].Centroid[i]
		cleanRightData = append(cleanRightData, v)
		rightCommits = append(rightCommits, cr.RightMostFrame.DataFrame.Header[i].Offset)
	}
	return cleanRightData, rightCommits
}

// calculateStepAndConfirm runs the selected step detection algorithm on the extracted
// left and right data, and checks if the result exceeds the interestingness threshold.
type improvedResult struct {
	isConfirmed   bool
	regressionVal float32
	stepSize      float32
	interesting   float32
}

func (r *ImprovedAnomalyBoundsRefiner) calculateStepAndConfirm(leftData, rightData []float32, rule *alerts.AnomalyDetectionRule) (float32, float32, float32, bool) {
	y0 := vec32.Mean(leftData)
	y1 := vec32.Mean(rightData)
	s1 := vec32.StdDev(leftData, y0)
	s2 := vec32.StdDev(rightData, y1)
	n1 := len(leftData)
	n2 := len(rightData)

	const CohenDVeryLarge = 1.2

	res := stepfit.TraverseRule(rule,
		// 1. Leaf node evaluation (Simple Rule)
		func(check *alerts.AlgorithmCheck) improvedResult {
			var regressionVal float32
			var stepSize float32
			var interesting float32

			algo := string(check.Step)
			threshold := check.Threshold

			switch algo {
			case string(types.AbsoluteStep):
				stepSize, regressionVal = stepfit.CalcAbsoluteStep(y0, y1)
				interesting = threshold
			case string(types.PercentStep):
				stepSize, regressionVal = stepfit.CalcPercentStep(y0, y1)
				interesting = threshold
			case string(types.CohenStep):
				stepSize, regressionVal = stepfit.CalcValidCohenStep(y0, y1, s1, s2, n1, n2, r.stdDevThreshold)
				interesting = min(threshold, float32(CohenDVeryLarge))
			default:
				// Fallback for algorithms that need different handling or are unsupported here
				stepSize, regressionVal = stepfit.CalcValidCohenStep(y0, y1, s1, s2, n1, n2, r.stdDevThreshold)
				interesting = CohenDVeryLarge
			}

			isConfirmed := math.Abs(float64(regressionVal)) >= float64(interesting)

			return improvedResult{isConfirmed, regressionVal, stepSize, interesting}
		},
		// 2. Combination logic (AND/OR)
		func(results []improvedResult, op string) improvedResult {
			var bools []bool
			for _, r := range results {
				bools = append(bools, r.isConfirmed)
			}

			isConfirmed := stepfit.CombineBooleans(bools, op)

			if len(results) == 0 {
				sklog.Warningf("[ImprovedAnomalyBoundsRefiner] Empty results slice in combine logic for operation: %q", op)
				return improvedResult{isConfirmed: isConfirmed}
			}

			// For regressionVal and stepSize (used for logging/metadata), we just pick the first one for simplicity.
			return improvedResult{isConfirmed, results[0].regressionVal, results[0].stepSize, results[0].interesting}
		})

	return res.regressionVal, res.stepSize, res.interesting, res.isConfirmed
}

// extractData extracts both the left and right side data points needed for the improved check.
// The left data is loaded from the trace store between the previous regression and the current one.
// The right data is extracted from the centroid of the right-most regression in the group.
// Returns nil slices to signal the caller to fallback to the original regression if not enough data is found.
func (r *ImprovedAnomalyBoundsRefiner) extractData(cr *regression.ConfirmedRegression, cfg *alerts.Alert, prevInfo *previousRegressionInfo, pData *prefetchedData) ([]float32, []types.CommitNumber, []float32, []types.CommitNumber) {
	traceName := cr.Summary.Clusters[0].Keys[0]
	leftData, leftCommits := pData.getLeftData(traceName, prevInfo.CommitNumber, cr.PrevCommitNumber, cfg.Radius)
	if len(leftData) < 3 {
		return nil, nil, nil, nil
	}

	rightData, rightCommits := extractRightData(cr)
	if len(rightData) < 3 {
		return nil, nil, nil, nil
	}

	return leftData, leftCommits, rightData, rightCommits
}

type prefetchedData struct {
	traces  map[string]types.Trace
	commits map[string][]types.CommitNumber
}

func (p *prefetchedData) getLeftData(traceName string, startCommit types.CommitNumber, endCommit types.CommitNumber, radius int) ([]float32, []types.CommitNumber) {
	if int(endCommit-startCommit+1) < radius {
		return nil, nil
	}
	traceData, ok := p.traces[traceName]
	if !ok {
		return nil, nil
	}
	commits := p.commits[traceName]

	var leftData []float32
	var leftCommits []types.CommitNumber
	for i, v := range traceData {
		c := commits[i]
		if c >= startCommit && c <= endCommit {
			leftData = append(leftData, v)
			leftCommits = append(leftCommits, c)
		}
	}

	// 200 is considered a reasonable number of points to capture the typical noise
	// and variance in the data for a reliable statistical check.
	const maxLeftDataPoints = 200
	if len(leftData) > maxLeftDataPoints {
		leftData = leftData[len(leftData)-maxLeftDataPoints:]
		leftCommits = leftCommits[len(leftCommits)-maxLeftDataPoints:]
	}

	return leftData, leftCommits
}
