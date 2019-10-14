package search

import (
	"context"
	"sort"
	"sync"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/tiling"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/golden/go/diff"
	"go.skia.org/infra/golden/go/indexer"
	"go.skia.org/infra/golden/go/search/common"
	"go.skia.org/infra/golden/go/search/frontend"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/summary"
	"go.skia.org/infra/golden/go/types"
)

const (
	// maxRowDigests is the maximum number of digests we'll compare against
	// before limiting the result to avoid overload.
	maxRowDigests = 200
)

// GetDigestTable implements the SearchAPI interface.
func (s *SearchImpl) GetDigestTable(q *query.DigestTable) (*frontend.DigestTable, error) {
	// Retrieve the row digests.
	idx := s.indexSource.GetIndex()
	rowDigests, err := s.filterTileCompare(q.RowQuery, idx)
	if err != nil {
		return nil, err
	}
	totalRowDigests := len(rowDigests)

	// Build the rows output.
	rows := getDTRows(rowDigests, q.SortRows, q.RowsDir, q.RowQuery.Limit, q.RowQuery.IgnoreState(), idx)

	// If the number exceeds the maximum we always sort and trim by frequency.
	if len(rows) > maxRowDigests {
		q.SortRows = query.SortByImageCounts
	}

	// If we sort by image frequency then we can sort and limit now, reducing the
	// number of diffs we need to make.
	sortEarly := q.SortRows == query.SortByImageCounts
	var uniqueTests types.TestNameSet = nil
	if sortEarly {
		uniqueTests = sortAndLimitRows(&rows, rowDigests, q.SortRows, q.RowsDir, q.Metric, q.RowQuery.Limit)
	}

	// Get the column digests conditioned on the result of the row digests.
	columnDigests, err := s.filterTileWithMatch(q.ColumnQuery, idx, q.Match, rowDigests)
	if err != nil {
		return nil, err
	}

	// Compare the rows in parallel.
	var wg sync.WaitGroup
	wg.Add(len(rows))
	rowLenCh := make(chan int, len(rows))
	for idx, rowElement := range rows {
		go func(idx int, digest types.Digest) {
			defer wg.Done()
			var total int
			var err error
			rows[idx].Values, total, err = getDiffs(s.diffStore, digest, columnDigests[digest].Keys(), q.ColumnsDir, q.Metric, q.ColumnQuery.Limit)
			if err != nil {
				sklog.Errorf("Unable to calculate diff of row for digest %s. Got error: %s", digest, err)
			}
			rowLenCh <- total
		}(idx, rowElement.Digest)
	}
	wg.Wait()

	// TODO(stephana): Add reference points (i.e. closest positive/negative, in trace)
	// to columns. Without these reference points the result only contains the
	// diff values.

	// Find the max length of rows and trim them if necessary.
	columns := []string{}
	columnsTotal := 0
	close(rowLenCh)
	for t := range rowLenCh {
		if t > columnsTotal {
			columnsTotal = t
		}
	}

	if !sortEarly {
		uniqueTests = sortAndLimitRows(&rows, rowDigests, q.SortRows, q.RowsDir, q.Metric, q.RowQuery.Limit)
	}

	// Get the summaries of all tests in the result.
	testSummaries := idx.GetSummaries(types.ExcludeIgnoredTraces)
	dtSummaries := make(map[types.TestName]*frontend.DTSummary, len(uniqueTests))
	for testName := range uniqueTests {
		dtSummaries[testName] = dtSummaryFromSummary(testSummaries[testName])
	}

	ret := &frontend.DigestTable{
		Grid: &frontend.DTGrid{
			Rows:         rows,
			RowsTotal:    totalRowDigests,
			Columns:      columns,
			ColumnsTotal: columnsTotal,
		},
		Corpus:    q.RowQuery.TraceValues.Get(types.CORPUS_FIELD),
		Summaries: dtSummaries,
	}

	return ret, nil
}

// filterTileCompare iterates over the tile and finds digests that match the given query.
// It returns a map[digest]ParamSet which contains all the found digests and
// the paramsets that generated them.
func (s *SearchImpl) filterTileCompare(q *query.Search, idx indexer.IndexSearcher) (map[types.Digest]paramtools.ParamSet, error) {
	ret := map[types.Digest]paramtools.ParamSet{}

	// Add digest/trace to the result.
	addFn := func(test types.TestName, digest types.Digest, traceID tiling.TraceId, trace *types.GoldenTrace, acceptRet interface{}) {
		if found, ok := ret[digest]; ok {
			found.AddParams(trace.Params())
		} else {
			ret[digest] = paramtools.NewParamSet(trace.Params())
		}
	}

	exp, err := s.expectationsStore.Get()
	if err != nil {
		return nil, err
	}

	if err := iterTile(q, addFn, nil, common.ExpSlice{exp}, idx); err != nil {
		return nil, err
	}
	return ret, nil
}

// paramsMatch Returns true if all the parameters listed in matchFields have matching values
// in condParamSets and params.
func paramsMatch(matchFields []string, condParamSets paramtools.ParamSet, params paramtools.Params) bool {
	for _, field := range matchFields {
		val, valOk := params[field]
		condVals, condValsOk := condParamSets[field]
		if !(valOk && condValsOk && util.In(val, condVals)) {
			return false
		}
	}
	return true
}

// filterTileWithMatch iterates over the tile and finds the digests that match
// the query and satisfy the condition of matching parameter values for the
// fields listed in matchFields. condDigests contains the digests their
// parameter sets for which we would like to find a set of digests for
// comparison. It returns a set of digests for each digest in condDigests.
func (s *SearchImpl) filterTileWithMatch(q *query.Search, idx indexer.IndexSearcher, matchFields []string, condDigests map[types.Digest]paramtools.ParamSet) (map[types.Digest]types.DigestSet, error) {
	if len(condDigests) == 0 {
		return map[types.Digest]types.DigestSet{}, nil
	}

	ret := make(map[types.Digest]types.DigestSet, len(condDigests))
	for d := range condDigests {
		ret[d] = types.DigestSet{}
	}

	// Define the acceptFn and addFn.
	var acceptFn AcceptFn = nil
	var addFn AddFn = nil
	if len(matchFields) >= 0 {
		matching := make(types.DigestSlice, 0, len(condDigests))
		acceptFn = func(params paramtools.Params, digests types.DigestSlice) (bool, interface{}) {
			matching = matching[:0]
			for digest, paramSet := range condDigests {
				if paramsMatch(matchFields, paramSet, params) {
					matching = append(matching, digest)
				}
			}
			return len(matching) > 0, matching
		}
		addFn = func(test types.TestName, digest types.Digest, traceID tiling.TraceId, trace *types.GoldenTrace, acceptRet interface{}) {
			for _, d := range acceptRet.(types.DigestSlice) {
				ret[d][digest] = true
			}
		}
	} else {
		addFn = func(test types.TestName, digest types.Digest, traceID tiling.TraceId, trace *types.GoldenTrace, acceptRet interface{}) {
			for d := range condDigests {
				ret[d][digest] = true
			}
		}
	}

	exp, err := s.expectationsStore.Get()
	if err != nil {
		return nil, err
	}

	if err := iterTile(q, addFn, acceptFn, common.ExpSlice{exp}, idx); err != nil {
		return nil, err
	}
	return ret, nil
}

// getDTRows returns the instance of DTRow that correspond to the given set of row digests.
func getDTRows(entries map[types.Digest]paramtools.ParamSet, sortField, sortDir string, limit int32, is types.IgnoreState, idx indexer.IndexSearcher) []*frontend.DTRow {
	talliesByTest := idx.DigestCountsByTest(is)
	ret := make([]*frontend.DTRow, 0, len(entries))
	for digest, paramSet := range entries {
		testName := types.TestName(paramSet[types.PRIMARY_KEY_FIELD][0])
		ret = append(ret, &frontend.DTRow{
			TestName: testName,
			DTDigestCount: frontend.DTDigestCount{
				Digest: digest,
				N:      talliesByTest[testName][digest],
			},
		})
	}
	return ret
}

// sortAndLimitRows sorts the given rows based on field, direction and diffMetric (if sorted by
// by diff). After the sort it will slice the result to be not larger than limit.
func sortAndLimitRows(rows *[]*frontend.DTRow, rowDigests map[types.Digest]paramtools.ParamSet, field, direction string, diffMetric string, limit int32) types.TestNameSet {
	// Determine the less function used for sorting the rows.
	var lessFn dtRowSliceLessFn
	if field == query.SortByImageCounts {
		lessFn = func(c *dtRowSlice, i, j int) bool { return c.data[i].N < c.data[j].N }
	} else if field == query.SortByDiff {
		lessFn = func(c *dtRowSlice, i, j int) bool {
			return (len(c.data[i].Values) > 0) && (len(c.data[j].Values) > 0) &&
				(c.data[i].Values[0].Diffs[diffMetric] < c.data[j].Values[0].Diffs[diffMetric])
		}
	}

	sortSlice := sort.Interface(newDTRowSlice(*rows, lessFn))
	if direction == query.SortDescending {
		sortSlice = sort.Reverse(sortSlice)
	}

	sort.Sort(sortSlice)
	lastIdx := util.MinInt32(limit, int32(len(*rows)))
	discarded := (*rows)[lastIdx:]
	for _, row := range discarded {
		delete(rowDigests, row.Digest)
	}
	*rows = (*rows)[:lastIdx]

	uniqueTests := types.TestNameSet{}
	for _, paramSets := range rowDigests {
		for _, t := range paramSets[types.PRIMARY_KEY_FIELD] {
			uniqueTests[types.TestName(t)] = true
		}
	}
	return uniqueTests
}

// Sort adapter to allow sorting rows by supplying a less function.
type dtRowSliceLessFn func(c *dtRowSlice, i, j int) bool
type dtRowSlice struct {
	lessFn dtRowSliceLessFn
	data   []*frontend.DTRow
}

func newDTRowSlice(data []*frontend.DTRow, lessFn dtRowSliceLessFn) *dtRowSlice {
	return &dtRowSlice{lessFn: lessFn, data: data}
}
func (c *dtRowSlice) Len() int           { return len(c.data) }
func (c *dtRowSlice) Less(i, j int) bool { return c.lessFn(c, i, j) }
func (c *dtRowSlice) Swap(i, j int)      { c.data[i], c.data[j] = c.data[j], c.data[i] }

// getDiffs gets the sorted and limited comparison of one digest against the list of digests.
// Arguments:
//    digest: primary digest
//    colDigests: the digests to compare against
//    sortDir: sort direction of the resulting list
//    diffMetric: id of the diffmetric to use (assumed to be defined in the diff package).
//    limit: is the maximum number of diffs to return after the sort.
func getDiffs(diffStore diff.DiffStore, digest types.Digest, colDigests types.DigestSlice, sortDir, diffMetric string, limit int32) ([]*frontend.DTDiffMetrics, int, error) {
	diffMap, err := diffStore.Get(context.TODO(), digest, colDigests)
	if err != nil {
		return nil, 0, err
	}

	ret := make([]*frontend.DTDiffMetrics, 0, len(diffMap))
	for colDigest, diffMetrics := range diffMap {
		ret = append(ret, &frontend.DTDiffMetrics{
			DiffMetrics: diffMetrics,
			DTDigestCount: frontend.DTDigestCount{
				Digest: colDigest,
				N:      0,
			},
		})
	}

	// TODO(stephana): Add the reference points for each row.

	lessFn := func(c *dtDiffMetricsSlice, i, j int) bool {
		return c.data[i].Diffs[diffMetric] < c.data[j].Diffs[diffMetric]
	}
	sortSlice := sort.Interface(newDTDiffMetricsSlice(ret, lessFn))
	if sortDir == query.SortDescending {
		sortSlice = sort.Reverse(sortSlice)
	}
	sort.Sort(sortSlice)
	return ret[:util.MinInt(int(limit), len(ret))], len(ret), nil
}

// Sort adapter to allow sorting lists of diff metrics via a less function.
type dtDiffMetricsSliceLessFn func(c *dtDiffMetricsSlice, i, j int) bool
type dtDiffMetricsSlice struct {
	lessFn dtDiffMetricsSliceLessFn
	data   []*frontend.DTDiffMetrics
}

func newDTDiffMetricsSlice(data []*frontend.DTDiffMetrics, lessFn dtDiffMetricsSliceLessFn) *dtDiffMetricsSlice {
	return &dtDiffMetricsSlice{lessFn: lessFn, data: data}
}
func (c *dtDiffMetricsSlice) Len() int           { return len(c.data) }
func (c *dtDiffMetricsSlice) Less(i, j int) bool { return c.lessFn(c, i, j) }
func (c *dtDiffMetricsSlice) Swap(i, j int)      { c.data[i], c.data[j] = c.data[j], c.data[i] }

func dtSummaryFromSummary(sum *summary.Summary) *frontend.DTSummary {
	return &frontend.DTSummary{
		Pos:       sum.Pos,
		Neg:       sum.Neg,
		Untriaged: sum.Untriaged,
	}
}
