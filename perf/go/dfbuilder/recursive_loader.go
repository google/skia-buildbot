package dfbuilder

import (
	"context"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"go.opencensus.io/trace"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/timer"
	"go.skia.org/infra/go/vec32"
	"go.skia.org/infra/perf/go/dataframe"
	"go.skia.org/infra/perf/go/progress"
	"go.skia.org/infra/perf/go/types"
	"golang.org/x/sync/errgroup"
)

const (
	// jaccardSimilarityThreshold defines the similarity threshold (Jaccard index)
	// used to group sparse traces into clusters for parallel history preloading.
	// See https://en.wikipedia.org/wiki/Jaccard_index
	jaccardSimilarityThreshold = 0.5

	// searchWindowSafetyMultiplier is the safety buffer multiplier applied to the
	// estimated commits needed to find the remaining data points in the base case.
	searchWindowSafetyMultiplier = 1.5

	// minTraceDensityFloor is the lower bound on trace density used when estimating search windows to prevent search window explosions.
	minTraceDensityFloor = 0.01

	// maxRecursionDepth limits the tree depth of recursive cluster splitting.
	maxRecursionDepth = 10

	// defaultQuerySemaphoreSize is the default concurrency limit for database queries per recursive load.
	defaultQuerySemaphoreSize = 10
)

type cluster struct {
	representative []int
	traceIDs       []string
}

type loaderMetrics struct {
	dbRequests    atomic.Int64
	totalDbTime   atomic.Int64
	groupsCreated atomic.Int64
	maxDepth      atomic.Int64
}

func (m *loaderMetrics) RecordDbQuery(duration time.Duration) {
	m.dbRequests.Add(1)
	m.totalDbTime.Add(int64(duration))
}

func (m *loaderMetrics) RecordGroups(count int) {
	m.groupsCreated.Add(int64(count))
}

func (m *loaderMetrics) RecordDepth(d int) {
	target := int64(d)
	for {
		current := m.maxDepth.Load()
		if target <= current {
			break
		}
		if m.maxDepth.CompareAndSwap(current, target) {
			break
		}
	}
}

// NewNFromKeysRecursive implements dataframe.DataFrameBuilder using parallel query-then-split recursion.
func (b *builder) NewNFromKeysRecursive(ctx context.Context, end time.Time, keys []string, n int32, progress progress.Progress, skipMetadata bool) (retDf *dataframe.DataFrame, finalErr error) {
	defer timer.NewWithSummary("perfserver_dfbuilder_NewNFromKeysRecursive", b.newNFromKeysRecursiveTimer).Stop()
	ctx, metrics, startTime, cancelSpan := b.initRecursiveContext(ctx)
	defer cancelSpan()

	endIndex, err := b.findIndexForTime(ctx, end)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to resolve end commit index")
	}
	if endIndex == types.BadCommitNumber {
		return dataframe.NewEmpty(), nil
	}

	initialStartCommit := endIndex - types.CommitNumber(b.tileSize) + 1
	if initialStartCommit < 0 {
		initialStartCommit = 0
	}
	sklog.Debugf("[NewNFromKeysRecursive] Start loading N=%d points for %d trace(s) ending at commit %d (initial search bounds: commit %d to %d)",
		n, len(keys), endIndex, initialStartCommit, endIndex)

	querySize := n
	requiredPoints := int(querySize)

	// Initial Bulk Query
	df, err := b.newNFromKeys(ctx, endIndex, keys, querySize, progress, b.tileSize, 0, false, skipMetadata)
	if err != nil {
		return nil, skerr.Wrapf(err, "Initial query failed")
	}
	if df == nil {
		return dataframe.NewEmpty(), nil
	}
	df = df.Compress()

	// Separate satisfied and deficient keys from initial load
	satisfiedKeys, deficientKeys := b.partitionKeysByDataCount(df, keys, requiredPoints)
	dfSatisfied := filterDataFrameByKeySet(df, satisfiedKeys)

	// Initialize tracking state
	initialScannedRange, tracker := b.initTrackingState(df, keys, endIndex)
	sklog.Debugf("[NewNFromKeysRecursive] Initial load result: %d trace(s) satisfied N=%d, %d trace(s) deficient (scanned_range: %d commits)",
		len(satisfiedKeys), requiredPoints, len(deficientKeys), initialScannedRange)

	// Cluster deficient traces initially based on initial df
	initialClusters := clusterDeficientTraces(ctx, df, deficientKeys, jaccardSimilarityThreshold)
	sklog.Debugf("[NewNFromKeysRecursive] Split %d deficient trace(s) into %d initial cluster(s)", len(deficientKeys), len(initialClusters))

	// Process initial clusters concurrently
	clusterDfs, err := b.processInitialClusters(ctx, df, initialClusters, endIndex, requiredPoints, initialScannedRange, tracker, progress, skipMetadata)
	if err != nil {
		return nil, err
	}

	// Merge all completed cluster dfs and the initial satisfied df
	allDfs := make([]*dataframe.DataFrame, 0, len(clusterDfs)+1)
	if len(satisfiedKeys) > 0 {
		allDfs = append(allDfs, dfSatisfied)
	}
	for _, dfC := range clusterDfs {
		if dfC != nil && len(dfC.Header) > 0 {
			allDfs = append(allDfs, dfC)
		}
	}

	dfMerged, err := dataframe.MultiJoin(ctx, allDfs...)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to merge dataframes recursively")
	}

	maskExcessPoints(dfMerged, requiredPoints)
	dfMerged = dfMerged.Compress()

	b.logRecursiveMetrics(metrics, startTime, len(keys))
	return dfMerged, nil
}

func (b *builder) initRecursiveContext(ctx context.Context) (context.Context, *loaderMetrics, time.Time, func()) {
	if _, ok := dataframe.QuerySemaphoreFromContext(ctx); !ok {
		localSem := make(chan struct{}, defaultQuerySemaphoreSize)
		ctx = dataframe.WithQuerySemaphore(ctx, localSem)
	}
	metrics := &loaderMetrics{}
	ctx = dataframe.WithLoaderMetrics(ctx, metrics)
	startTime := time.Now()
	ctx, span := trace.StartSpan(ctx, "dfbuilder.NewNFromKeysRecursive")
	return ctx, metrics, startTime, span.End
}

func (b *builder) partitionKeysByDataCount(df *dataframe.DataFrame, keys []string, requiredPoints int) (satisfiedKeys, deficientKeys []string) {
	for _, id := range keys {
		if b.countNonMissing(df, id) >= requiredPoints {
			satisfiedKeys = append(satisfiedKeys, id)
		} else {
			deficientKeys = append(deficientKeys, id)
		}
	}
	return satisfiedKeys, deficientKeys
}

func (b *builder) processInitialClusters(ctx context.Context, df *dataframe.DataFrame, initialClusters []cluster, endIndex types.CommitNumber, requiredPoints int, initialScannedRange int32, tracker *recursiveTracker, progress progress.Progress, skipMetadata bool) ([]*dataframe.DataFrame, error) {
	g, gCtx := errgroup.WithContext(ctx)
	clusterDfs := make([]*dataframe.DataFrame, len(initialClusters))

	for idx, clusterVal := range initialClusters {
		i, c := idx, clusterVal
		g.Go(func() error {
			dfLocal := filterDataFrameByKeySet(df, c.traceIDs)
			label := fmt.Sprintf("initial.%d", i+1)

			finalDfLocal, err := b.processClusterRecursive(gCtx, label, c.traceIDs, dfLocal, dfLocal, endIndex, requiredPoints, initialScannedRange, tracker, progress, 0, skipMetadata)
			if err != nil {
				return err
			}
			clusterDfs[i] = finalDfLocal
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}
	return clusterDfs, nil
}

func (b *builder) logRecursiveMetrics(metrics *loaderMetrics, startTime time.Time, totalKeys int) {
	reqs := metrics.dbRequests.Load()
	totTime := metrics.totalDbTime.Load()
	groups := metrics.groupsCreated.Load()
	maxD := metrics.maxDepth.Load()

	metrics2.GetCounter("perfserver_dfbuilder_newNFromKeysRecursive_requests").Inc(1)
	metrics2.GetCounter("perfserver_dfbuilder_newNFromKeysRecursive_db_requests").Inc(reqs)
	metrics2.GetCounter("perfserver_dfbuilder_newNFromKeysRecursive_groups_created").Inc(groups)
	metrics2.GetFloat64SummaryMetric("perfserver_dfbuilder_newNFromKeysRecursive_max_depth").Observe(float64(maxD))
	metrics2.GetFloat64SummaryMetric("perfserver_dfbuilder_newNFromKeysRecursive_db_time_s").Observe(time.Duration(totTime).Seconds())

	var avgDbTime time.Duration
	if reqs > 0 {
		avgDbTime = time.Duration(totTime / reqs)
	}
	sklog.Debugf("[NewNFromKeysRecursive] Completed in %v for %d traces. Summary: db_requests=%d, avg_db_duration=%v, total_groups=%d, max_depth=%d",
		time.Since(startTime), totalKeys, reqs, avgDbTime, groups, maxD)
}

type recursiveTracker struct {
	mu              sync.RWMutex
	exhaustedTraces map[string]bool
}

func newRecursiveTracker() *recursiveTracker {
	return &recursiveTracker{
		exhaustedTraces: make(map[string]bool),
	}
}

func (t *recursiveTracker) isExhausted(key string) bool {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.exhaustedTraces[key]
}

func (t *recursiveTracker) setExhausted(key string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.exhaustedTraces[key] = true
}

func (b *builder) initTrackingState(df *dataframe.DataFrame, traceIds []string, endIndex types.CommitNumber) (int32, *recursiveTracker) {
	tracker := newRecursiveTracker()

	initialScannedRange := int32(b.tileSize)
	if len(df.Header) > 0 && endIndex != types.BadCommitNumber {
		headerRange := int32(endIndex - df.Header[0].Offset + 1)
		if headerRange > initialScannedRange {
			initialScannedRange = headerRange
		}
		if df.Header[0].Offset <= 0 {
			for _, id := range traceIds {
				tracker.setExhausted(id)
			}
		}
	}
	return initialScannedRange, tracker
}

func (b *builder) findDeficientTraces(df *dataframe.DataFrame, traceIds []string, tracker *recursiveTracker, requiredPoints int) []string {
	deficient := []string{}
	for _, id := range traceIds {
		if tracker.isExhausted(id) {
			continue
		}
		if b.countNonMissing(df, id) < requiredPoints {
			deficient = append(deficient, id)
		}
	}
	return deficient
}

func (b *builder) countNonMissing(df *dataframe.DataFrame, traceID string) int {
	traceValues := df.TraceSet[traceID]
	count := 0
	for _, val := range traceValues {
		if val != vec32.MissingDataSentinel {
			count++
		}
	}
	return count
}

func (b *builder) calculateClusterSearchWindow(df *dataframe.DataFrame, groupKeys []string, prevRange int32, requiredPoints int) int32 {
	minTileSize := int(b.tileSize)
	totalSearchCommits := 0
	count := 0

	for _, traceID := range groupKeys {
		traceValues := df.TraceSet[traceID]
		nonMissingCount := 0
		for _, val := range traceValues {
			if val != vec32.MissingDataSentinel {
				nonMissingCount++
			}
		}

		neededPoints := requiredPoints - nonMissingCount
		if neededPoints <= 0 {
			continue
		}

		var searchCommits int
		headerLen := len(df.Header)
		if headerLen == 0 {
			searchCommits = int(prevRange)
		} else if nonMissingCount == 0 {
			searchCommits = int(prevRange * 2)
		} else {
			density := float64(nonMissingCount) / float64(headerLen)
			if density < minTraceDensityFloor {
				density = minTraceDensityFloor
			}
			estimatedCommits := float64(neededPoints) / density
			searchCommits = int(estimatedCommits * searchWindowSafetyMultiplier)
		}

		if searchCommits < minTileSize {
			searchCommits = minTileSize
		}

		totalSearchCommits += searchCommits
		count++
	}

	if count == 0 {
		return int32(minTileSize)
	}

	avgSearchCommits := totalSearchCommits / count
	if avgSearchCommits < minTileSize {
		avgSearchCommits = minTileSize
	}

	return int32(avgSearchCommits)
}

func (b *builder) updateTrackingState(dfExt *dataframe.DataFrame, groupKeys []string, targetCommit types.CommitNumber, tracker *recursiveTracker) *dataframe.DataFrame {
	if dfExt == nil {
		for _, key := range groupKeys {
			tracker.setExhausted(key)
		}
		return nil
	}

	dfClean := dfExt.Compress()
	if dfClean == nil || len(dfClean.Header) == 0 {
		for _, key := range groupKeys {
			tracker.setExhausted(key)
		}
		return nil
	} else if targetCommit <= 0 || dfClean.Header[0].Offset <= 0 {
		for _, key := range groupKeys {
			tracker.setExhausted(key)
		}
	}

	return dfClean
}

// jaccardSimilarity calculates the Jaccard similarity coefficient (|A ∩ B| / |A ∪ B|) between two sorted slices.
// See https://en.wikipedia.org/wiki/Jaccard_index
func jaccardSimilarity(a, b []int) float64 {
	if len(a) == 0 && len(b) == 0 {
		return 1.0
	}
	if len(a) == 0 || len(b) == 0 {
		return 0.0
	}
	intersection := 0
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] == b[j] {
			intersection++
			i++
			j++
		} else if a[i] < b[j] {
			i++
		} else {
			j++
		}
	}
	union := len(a) + len(b) - intersection
	return float64(intersection) / float64(union)
}

// clusterDeficientTraces groups deficient traces into signature clusters based on the Jaccard similarity
// (https://en.wikipedia.org/wiki/Jaccard_index) of their present (existing) commit patterns. Traces are sorted descending
// by non-missing point count before clustering to ensure stable representatives and deterministic grouping.
func clusterDeficientTraces(ctx context.Context, df *dataframe.DataFrame, deficientTraces []string, threshold float64) []cluster {
	if len(deficientTraces) == 0 {
		return nil
	}

	sortedTraces := sortTracesByDataCount(df, deficientTraces)
	clusters := groupTracesIntoClusters(df, sortedTraces, threshold)

	if mVal, ok := dataframe.LoaderMetricsFromContext(ctx); ok {
		if m, ok := mVal.(*loaderMetrics); ok {
			m.RecordGroups(len(clusters))
		}
	}

	return clusters
}

func sortTracesByDataCount(df *dataframe.DataFrame, traces []string) []string {
	tracePointCounts := make(map[string]int, len(traces))
	for _, traceID := range traces {
		traceValues := df.TraceSet[traceID]
		count := 0
		for _, val := range traceValues {
			if val != vec32.MissingDataSentinel {
				count++
			}
		}
		tracePointCounts[traceID] = count
	}

	sortedTraces := make([]string, len(traces))
	copy(sortedTraces, traces)
	sort.Slice(sortedTraces, func(i, j int) bool {
		cI := tracePointCounts[sortedTraces[i]]
		cJ := tracePointCounts[sortedTraces[j]]
		if cI != cJ {
			return cI > cJ // Descending order
		}
		return sortedTraces[i] < sortedTraces[j] // Deterministic tie-breaker
	})
	return sortedTraces
}

func extractPresentIndices(tr types.Trace) []int {
	present := make([]int, 0, len(tr))
	for i, val := range tr {
		if val != vec32.MissingDataSentinel {
			present = append(present, i)
		}
	}
	return present
}

func groupTracesIntoClusters(df *dataframe.DataFrame, sortedTraces []string, threshold float64) []cluster {
	var clusters []cluster
	for _, traceID := range sortedTraces {
		present := extractPresentIndices(df.TraceSet[traceID])

		merged := false
		for i, c := range clusters {
			if jaccardSimilarity(present, c.representative) >= threshold {
				clusters[i].traceIDs = append(clusters[i].traceIDs, traceID)
				merged = true
				break
			}
		}
		if !merged {
			clusters = append(clusters, cluster{
				representative: present,
				traceIDs:       []string{traceID},
			})
		}
	}
	return clusters
}

func maskExcessPoints(df *dataframe.DataFrame, n int) {
	for _, tr := range df.TraceSet {
		count := 0
		for i := len(tr) - 1; i >= 0; i-- {
			if tr[i] != vec32.MissingDataSentinel {
				count++
				if count > n {
					tr[i] = vec32.MissingDataSentinel
				}
			}
		}
	}
}

// processClusterRecursive processes a single cluster of traces recursively.
// It performs queries to satisfy the traces in clusterKeys, splits them further if they remain deficient,
// and recursively resolves older history.
func (b *builder) processClusterRecursive(
	ctx context.Context,
	label string,
	clusterKeys []string,
	dfLocal *dataframe.DataFrame,
	dfLatest *dataframe.DataFrame,
	endIndex types.CommitNumber,
	requiredPoints int,
	scannedRange int32,
	tracker *recursiveTracker,
	progress progress.Progress,
	depth int,
	skipMetadata bool,
) (*dataframe.DataFrame, error) {
	if mVal, ok := dataframe.LoaderMetricsFromContext(ctx); ok {
		if m, ok := mVal.(*loaderMetrics); ok {
			m.RecordDepth(depth)
		}
	}

	if depth >= maxRecursionDepth {
		return nil, skerr.Fmt("max recursion depth %d reached for cluster label %s with %d trace(s); consider increasing search window or changing trace grouping strategy", maxRecursionDepth, label, len(clusterKeys))
	}

	// Check which traces in the cluster are still deficient using merged dfLocal
	deficientTraces := b.findDeficientTraces(dfLocal, clusterKeys, tracker, requiredPoints)

	if len(deficientTraces) == 0 {
		sklog.Debugf("[NewNFromKeysRecursive][%s] All %d trace(s) satisfied at depth %d", label, len(clusterKeys), depth)
		return dfLocal, nil
	}

	prevRange := scannedRange

	maxNeeded := 0
	for _, id := range deficientTraces {
		needed := requiredPoints - b.countNonMissing(dfLocal, id)
		if needed > maxNeeded {
			maxNeeded = needed
		}
	}

	// Calculate window size using only dfLatest (the latest step's DataFrame)
	searchWindow := b.calculateClusterSearchWindow(dfLatest, deficientTraces, prevRange, requiredPoints)
	targetCommit := endIndex - types.CommitNumber(prevRange)
	if targetCommit <= 0 {
		targetCommit = 0
	}

	sklog.Debugf("[NewNFromKeysRecursive][%s] Step start (depth=%d, total_keys=%d, scanned_range=%d). Querying older history ending at commit %d (search_window=%d, max_needed=%d) for %d deficient trace(s)",
		label, depth, len(clusterKeys), prevRange, targetCommit, searchWindow, maxNeeded, len(deficientTraces))

	dfExt, err := b.newNFromKeys(ctx, targetCommit, deficientTraces, int32(maxNeeded), progress, searchWindow, 0, false, skipMetadata)
	if err != nil {
		return nil, err
	}

	dfClean := b.updateTrackingState(dfExt, deficientTraces, targetCommit, tracker)
	nextScannedRange := prevRange + searchWindow

	// If no data was returned, all deficient traces in this cluster are exhausted
	if dfClean == nil {
		sklog.Debugf("[NewNFromKeysRecursive][%s] All %d deficient trace(s) exhausted at target_commit %d", label, len(deficientTraces), targetCommit)
		return dfLocal, nil
	}

	sklog.Debugf("[NewNFromKeysRecursive][%s] Range query result: returned %d header column(s) spanning [%d .. %d]",
		label, len(dfClean.Header), dfClean.Header[0].Offset, dfClean.Header[len(dfClean.Header)-1].Offset)

	// Merge newly loaded data into our local group DataFrame
	dfMergedLocally, err := dataframe.MultiJoin(ctx, dfLocal, dfClean)
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to merge dfLocal and dfClean")
	}

	// Find if any traces are STILL deficient after this query
	remainingDeficient := b.findDeficientTraces(dfMergedLocally, deficientTraces, tracker, requiredPoints)

	if len(remainingDeficient) == 0 {
		sklog.Debugf("[NewNFromKeysRecursive][%s] All %d trace(s) satisfied after range query at target_commit %d", label, len(clusterKeys), targetCommit)
		return dfMergedLocally, nil
	}

	// Split the remaining deficient traces into sub-clusters based on their missing/present patterns
	// using only dfClean (the latest step's DataFrame)
	subClusters := clusterDeficientTraces(ctx, dfClean, remainingDeficient, jaccardSimilarityThreshold)
	sklog.Debugf("[NewNFromKeysRecursive][%s] %d trace(s) completed N=%d; %d trace(s) still deficient -> splitting into %d sub-group(s) for further range queries",
		label, len(deficientTraces)-len(remainingDeficient), requiredPoints, len(remainingDeficient), len(subClusters))

	g, gCtx := errgroup.WithContext(ctx)
	subDfs := make([]*dataframe.DataFrame, len(subClusters))

	for idx, scVal := range subClusters {
		i, c := idx, scVal
		g.Go(func() error {
			groupKeys := c.traceIDs

			// Recursively call on the sub-cluster with a sub-label (e.g. "initial.1" -> "initial.1.1")
			subLabel := fmt.Sprintf("%s.%d", label, i+1)
			sklog.Debugf("[NewNFromKeysRecursive][%s -> %s] Sub-group created with %d trace(s)", label, subLabel, len(groupKeys))

			// Filter dfMergedLocally to only contain the sub-cluster's groupKeys to prevent sibling data overwrites
			dfLocalSub := filterDataFrameByKeySet(dfMergedLocally, groupKeys)

			// We pass dfLocalSub as the merged dfLocal, and dfClean as the latest step dfLatest.
			dfRec, err := b.processClusterRecursive(gCtx, subLabel, groupKeys, dfLocalSub, dfClean, endIndex, requiredPoints, nextScannedRange, tracker, progress, depth+1, skipMetadata)
			if err != nil {
				return err
			}
			subDfs[i] = dfRec
			return nil
		})
	}
	if err := g.Wait(); err != nil {
		return nil, err
	}

	extendedDfs := append([]*dataframe.DataFrame{dfMergedLocally}, subDfs...)
	return dataframe.MultiJoin(ctx, extendedDfs...)
}

// filterDataFrameByKeySet returns a new DataFrame containing only the traces in the keys slice.
func filterDataFrameByKeySet(df *dataframe.DataFrame, keys []string) *dataframe.DataFrame {
	ret := dataframe.NewEmpty()
	ret.Header = df.Header
	ret.ParamSet = df.ParamSet
	ret.Skip = df.Skip
	ret.SourceInfo = make(map[string]*types.TraceSourceInfo, len(keys))
	for _, key := range keys {
		ret.TraceSet[key] = df.TraceSet[key]
		if info, ok := df.SourceInfo[key]; ok {
			ret.SourceInfo[key] = info
		}
	}
	ret.BuildParamSet()
	return ret
}
