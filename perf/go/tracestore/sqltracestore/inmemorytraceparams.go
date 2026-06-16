package sqltracestore

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/jackc/pgx/v4"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
)

type InMemoryTraceParams struct {
	db                       pool.Pool
	refreshIntervalInSeconds float32
	showOnlyPublicTraces     bool

	// Array of column data. Each column is an array containing integer encodings
	// of strings for a single param in the original traceparams table. The column
	// arrays should all the be same length, and with elements in the same order,
	// such that for the i-th traceparams row, {traceparams[0][i], traceparams[1][i],
	// ..., traceparams[n][i]} gives the encoded param values.
	traceparams [][]int32

	// Map column names (aka. param keys) to index in array of columns (traceparams)
	paramCols map[string]int32
	// Map index in array of columns (traceparams) to column names (aka. param keys)
	colParams map[int32]string

	// Map strings (aka. param values) to integer encodings
	encoding map[string]int32
	// Map integer encodings to strings (aka. param values)
	rEncoding map[int32]string

	// Inverted index: map[param_key]map[param_value_encoding][]int32
	// Slices of trace_id_idx are naturally sorted because they are built in order.
	invertIndex map[string]map[int32][]int32

	// Lock around "in-memory" data. Querying takes RLock (non-exclusive),
	// refreshing takes Lock (exclusive).
	dataLock sync.RWMutex

	// publicTraceIDs is a pointer-free map containing raw trace ID MD5 bytes.
	// It is only allocated and populated when showOnlyPublicTraces is true to prevent GC/memory waste.
	publicTraceIDs map[types.TraceIDForSQLInBytes]struct{}

	// publicParamSet holds the pre-computed ReadOnlyParamSet when showOnlyPublicTraces is true.
	publicParamSet paramtools.ReadOnlyParamSet
}

type ContextKey string

const UseInvertedIndex ContextKey = "useInvertedIndex"
const AllowEmptyQuery ContextKey = "allowEmptyQuery"

const (
	// No of partitions to use when reading traceParams table.
	traceParamsReadPartitionCount = 16
	// Timeout for each worker reading a partition.
	traceParamsWorkerTimeout = 5 * time.Minute
)

func (tp *InMemoryTraceParams) Refresh(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "InMemoryTraceParams.refresh")
	defer span.End()
	traceparams := [][]int32{}
	paramCols := map[string]int32{}
	colParams := map[int32]string{}

	// Get latest tilenumber
	const getLatestTile = `
		SELECT
			tile_number
		FROM
			paramsets
		ORDER BY
			tile_number DESC
		LIMIT
			1;
		`
	tileNumber := types.BadTileNumber
	if err := tp.db.QueryRow(ctx, getLatestTile).Scan(&tileNumber); err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return skerr.Wrap(err)
	}

	// Get traceparams columns
	const paramSetForTile = `
		SELECT
			DISTINCT param_key
		FROM
			paramsets
		WHERE
			tile_number = $1 OR tile_number = $1-1;
		`
	paramsRows, err := tp.db.Query(ctx, paramSetForTile, tileNumber)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil
		}
		return skerr.Wrap(err)
	}
	defer paramsRows.Close()
	var pCount int32 = 0
	for paramsRows.Next() {
		var key string
		if err := paramsRows.Scan(&key); err != nil {
			return skerr.Wrapf(err, "Failed scanning row - tileNumber=%d", tileNumber)
		}
		paramCols[key] = pCount
		colParams[pCount] = key
		pCount++
		traceparams = append(traceparams, []int32{})
	}

	// Get traceparams row data
	traceparams, publicTraceIDs, encoding, rEncoding, err := tp.readAllTraceParams(ctx, traceparams, paramCols)
	if err != nil {
		return skerr.Wrap(err)
	}

	var publicParamSet paramtools.ReadOnlyParamSet
	if tp.showOnlyPublicTraces {
		publicParamSet = buildParamSet(traceparams, paramCols, rEncoding).Freeze()
	}

	invertIndex := buildInvertedIndex(paramCols, traceparams)

	tp.dataLock.Lock()
	defer tp.dataLock.Unlock()
	tp.traceparams = traceparams
	tp.paramCols = paramCols
	tp.colParams = colParams
	tp.encoding = encoding
	tp.rEncoding = rEncoding
	tp.invertIndex = invertIndex
	tp.publicTraceIDs = publicTraceIDs
	tp.publicParamSet = publicParamSet
	return nil
}

// Struct to hold the row data
type TraceParam struct {
	TraceID []byte
	Params  map[string]interface{}
}

// readAllTraceParams reads the entire traceparams table and populates the data in the necessary
// structures.
//
// Step 1: Start a goroutine that processes each row being read from the traceparams table.
// Step 2: Create workers to read the table in parallel and report the rows to the traceparams
// channel.
// Step 3: Wait until all workers are done, and all rows are processed.
//
// We partition the table data based on the hex representation of the traceId and provide each
// worker with a unique partition (i.e range of traceIds or rows) to read. This helps speed up
// the reading of the entire table by running it in parallel without workers overlapping each other.
func (tp *InMemoryTraceParams) readAllTraceParams(ctx context.Context, traceparams [][]int32, paramCols map[string]int32) ([][]int32, map[types.TraceIDForSQLInBytes]struct{}, map[string]int32, map[int32]string, error) {
	var wg sync.WaitGroup
	var traceParamReadWg sync.WaitGroup
	totalRows := int64(0)
	errors := make(chan error, traceParamsReadPartitionCount)
	traceParamsChan := make(chan TraceParam, 10000)
	encoding := map[string]int32{}
	rEncoding := map[int32]string{}
	var publicTraceIDs map[types.TraceIDForSQLInBytes]struct{}
	if tp.showOnlyPublicTraces {
		publicTraceIDs = map[types.TraceIDForSQLInBytes]struct{}{}
	}

	var traceIdCount int = 0
	var eCount int32 = 0

	totalRowMutex := sync.Mutex{}

	// Start a goroutine that will read the traceparam entries being published by the workers.
	traceParamReadWg.Add(1)
	go func() {
		defer traceParamReadWg.Done()
		for traceParam := range traceParamsChan {
			if tp.showOnlyPublicTraces {
				var traceIDArray types.TraceIDForSQLInBytes
				copy(traceIDArray[:], traceParam.TraceID)
				publicTraceIDs[traceIDArray] = struct{}{}
			}

			for param, paramCol := range paramCols {
				var valueE int32 = -1
				value, ok := traceParam.Params[param]
				if ok {
					valueString := value.(string)
					valueE, ok = encoding[valueString]
					if !ok {
						// If string hasn't been encoded yet then map param values to number encoding, and back.
						valueE = eCount
						encoding[valueString] = valueE
						rEncoding[valueE] = valueString
						eCount++
					}
				}
				// Store encoded value for this row in column
				traceparams[paramCol] = append(traceparams[paramCol], valueE)
			}
			traceIdCount++
		}
	}()

	sklog.Infof("Starting partitioned read with %d workers...", traceParamsReadPartitionCount)

	for i := 0; i < traceParamsReadPartitionCount; i++ {
		wg.Add(1)

		// Calculate the start and end of the hex range for this partition (e.g., 0x00 to 0x0F, 0x10 to 0x1F)
		startByte := fmt.Sprintf("%02X", i*256/traceParamsReadPartitionCount)     // 00, 10, 20, ... F0 (for 16 partitions)
		endByte := fmt.Sprintf("%02X", (i+1)*256/traceParamsReadPartitionCount-1) // 0F, 1F, 2F, ... FF

		// Create the full key range boundary conditions
		// Note: The BETWEEN operator includes both bounds.
		// For BYTEA, this is a lexicographical comparison.
		lowerBound := startByte + "000000000000000000000000000000"
		upperBound := endByte + "FFFFFFFFFFFFFFFFFFFFFFFFFFFFFFFF"

		// Decode hex to byte slices for correct SQL comparison
		lowerBytes, _ := hex.DecodeString(lowerBound)
		upperBytes, _ := hex.DecodeString(upperBound)

		go func(workerID int, lower []byte, upper []byte) {
			defer wg.Done()

			// The SQL query uses the BETWEEN operator on the BYTEA primary key
			sqlQuery := `
				SELECT trace_id, params
				FROM TraceParams
				WHERE trace_id >= $1 AND trace_id <= $2
			`
			if tp.showOnlyPublicTraces {
				sqlQuery += " AND is_public = TRUE"
			}

			workerCtx, cancel := context.WithTimeout(ctx, traceParamsWorkerTimeout)
			defer cancel()
			rows, err := tp.db.Query(workerCtx, sqlQuery, lower, upper)
			if err != nil {
				if err != pgx.ErrNoRows {
					errors <- skerr.Fmt("worker %d query failed for range %x-%x: %w", workerID, lower, upper, err)
				}

				return
			}
			defer rows.Close()

			partitionRows := 0

			for rows.Next() {
				var traceParamRow TraceParam
				if err := rows.Scan(&traceParamRow.TraceID, &traceParamRow.Params); err != nil {
					errors <- skerr.Fmt("worker %d failed to scan row: %w", workerID, err)
					return
				}

				// Publish the row data on the traceParams channel.
				traceParamsChan <- traceParamRow
				partitionRows++
			}

			if rows.Err() != nil {
				errors <- skerr.Fmt("worker %d iteration error: %w", workerID, rows.Err())
				return
			}

			sklog.Infof("Worker %d finished. Read %d rows in range %x-%x.", workerID, partitionRows, lower[:2], upper[:2])
			totalRowMutex.Lock()
			defer totalRowMutex.Unlock()
			totalRows += int64(partitionRows)

		}(i, lowerBytes, upperBytes)
	}

	// Wait for all workers to finish
	wg.Wait()
	close(errors)
	close(traceParamsChan)

	sklog.Infof("Ensuring all data has been read on traceparams channel.")
	traceParamReadWg.Wait()

	// Check for any errors
	for err := range errors {
		return nil, nil, nil, nil, err // Return the first error encountered
	}

	sklog.Infof("TraceParams Partitioned Read complete. Total estimated rows processed: %d", totalRows)
	return traceparams, publicTraceIDs, encoding, rEncoding, nil
}

func (tp *InMemoryTraceParams) startRefresher(ctx context.Context) error {
	// Initialize
	err := tp.Refresh(ctx)
	if err != nil {
		return err
	}

	// Update the cache periodically.
	go func() {
		// Periodically run it based on the specified duration.
		refreshDuration := time.Second * time.Duration(tp.refreshIntervalInSeconds)
		for range time.Tick(refreshDuration) {
			err := tp.Refresh(ctx)
			if err != nil {
				sklog.Errorf("Error updating alert configurations. %s", err)
			}
		}
	}()

	return nil
}

// Create a new InMemoryTraceParams and populate it with data from traceparams table in db
func NewInMemoryTraceParams(ctx context.Context, db pool.Pool, refreshIntervalInSeconds float32, showOnlyPublicTraces bool) (*InMemoryTraceParams, error) {
	ctx, span := trace.StartSpan(ctx, "InMemoryTraceParams.NewInMemoryTraceParams")
	defer span.End()

	ret := InMemoryTraceParams{
		db:                       db,
		refreshIntervalInSeconds: refreshIntervalInSeconds,
		showOnlyPublicTraces:     showOnlyPublicTraces,
	}
	err := ret.startRefresher(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &ret, nil
}

// TraceAccessAllowed returns true if the traceName is public or visibility filtering is disabled.
func (tp *InMemoryTraceParams) TraceAccessAllowed(traceName string) bool {
	if !tp.showOnlyPublicTraces {
		return true
	}
	tp.dataLock.RLock()
	defer tp.dataLock.RUnlock()
	if tp.publicTraceIDs == nil {
		return false
	}
	traceIDBytes := types.TraceIDForSQLInBytesFromTraceName(traceName)
	_, ok := tp.publicTraceIDs[traceIDBytes]
	return ok
}

// Query for paramsets in (in-memory version of) traceparams table matching query input
func (tp *InMemoryTraceParams) QueryTraceIDs(ctx context.Context, tileNumber types.TileNumber,
	q *query.Query, outParams chan paramtools.Params) {
	if ctx.Value(UseInvertedIndex) == true {
		tp.queryTraceIDsInvertIndex(ctx, tileNumber, q, outParams)
		return
	}

	go func() {
		sklog.Debug("Start filtering encodings")
		ctx, span := trace.StartSpan(ctx, "InMemoryTraceParams.QueryTraceIDs")
		defer span.End()
		defer close(outParams)
		tp.dataLock.RLock()
		defer tp.dataLock.RUnlock()
		if len(tp.traceparams) == 0 {
			return
		}
		numTraceparams := len(tp.traceparams[0])
		const kChunkSize int = 2000
		const kTraceIdQueryPoolSize int = 30
		err := util.ChunkIterParallelPool(ctx, numTraceparams, kChunkSize, kTraceIdQueryPoolSize,
			func(ctx context.Context, startIdx, endIdx int) error {
				var traceids [kChunkSize]int
				traceidCount := endIdx - startIdx
				// Fill traceids with all the indexes between startIdx and endIdx:
				for i := range traceidCount {
					traceids[i] = startIdx + i
				}
				// Iterate over queryParams, narrowing down set of traceIds each iteration
				for _, queryParam := range q.Params {
					key := queryParam.Key()
					colIndex, ok := tp.paramCols[key]
					if !ok {
						traceidCount = 0
						break
					}
					column := tp.traceparams[colIndex]

					// Encode positive param values
					var eValues []int32 = []int32{}
					for _, paramValue := range queryParam.Values {
						e, ok := tp.encoding[paramValue]
						if ok {
							eValues = append(eValues, e)
						}
					}

					// Encode negative param values
					var eNegativeValues []int32 = []int32{}
					for _, paramValue := range queryParam.NegativeValues {
						e, ok := tp.encoding[paramValue]
						if ok {
							eNegativeValues = append(eNegativeValues, e)
						}
					}

					hasPositiveConstraints := len(queryParam.Values) > 0 || queryParam.Reg != nil

					// Iterate over traceids and keep them only if they match the query
					nextTraceidCount := 0
					for i := range traceidCount {
						traceIdIdx := traceids[i]
						val := column[traceIdIdx]

						if val == -1 {
							continue
						}

						// Check negative values (exclusion)
						excluded := false
						for _, eNeg := range eNegativeValues {
							if val == eNeg {
								excluded = true
								break
							}
						}
						if excluded {
							continue
						}

						// String decoding lazily
						var valStr string
						valStrDecoded := false

						if queryParam.NegativeReg != nil {
							valStr = tp.rEncoding[val]
							valStrDecoded = true
							if queryParam.NegativeReg.MatchString(valStr) {
								continue
							}
						}

						// Check positive constraints
						if hasPositiveConstraints {
							matched := false
							// Check positive values
							for _, ePos := range eValues {
								if val == ePos {
									matched = true
									break
								}
							}
							// Check positive regex
							if !matched && queryParam.Reg != nil {
								if !valStrDecoded {
									valStr = tp.rEncoding[val]
									valStrDecoded = true
								}
								if queryParam.Reg.MatchString(valStr) {
									matched = true
								}
							}

							if !matched {
								continue
							}
						}

						traceids[nextTraceidCount] = traceIdIdx
						nextTraceidCount++
					}
					traceidCount = nextTraceidCount
				}

				// Unencode traceparams rows back to strings
				for i := range traceidCount {
					traceIdIdx := traceids[i]
					var params paramtools.Params = paramtools.Params{}
					for p, c := range tp.paramCols {
						e := tp.traceparams[c][traceIdIdx]
						if e >= 0 {
							params[p] = tp.rEncoding[tp.traceparams[c][traceIdIdx]]
						}
					}
					outParams <- params
				}

				return nil
			})
		if err != nil {
			sklog.Errorf("Error querying traceids. %s", err)
		}
	}()
}

func (tp *InMemoryTraceParams) queryTraceIDsInvertIndex(ctx context.Context, tileNumber types.TileNumber,
	q *query.Query, outParams chan paramtools.Params) {
	go func() {
		sklog.Debug("Start filtering encodings")
		_, span := trace.StartSpan(ctx, "InMemoryTraceParams.QueryTraceIDs")
		defer span.End()
		defer close(outParams)
		tp.dataLock.RLock()
		defer tp.dataLock.RUnlock()

		matchedIndices := tp.executeQuery(q)

		sklog.Infof("Found %d matching traces", len(matchedIndices))

		// Reconstruct Params for matching traces
		for _, traceIdIdx := range matchedIndices {
			var params paramtools.Params = paramtools.Params{}
			for p, c := range tp.paramCols {
				e := tp.traceparams[c][traceIdIdx]
				if e >= 0 {
					params[p] = tp.rEncoding[e]
				}
			}
			outParams <- params
		}
	}()
}

func (tp *InMemoryTraceParams) executeQuery(q *query.Query) []int32 {
	if len(tp.traceparams) == 0 {
		return nil
	}

	var currentResult []int32
	first := true

	initAllTraces := func() {
		numTraces := len(tp.traceparams[0])
		currentResult = make([]int32, numTraces)
		for i := range currentResult {
			currentResult[i] = int32(i)
		}
		first = false
	}

	for _, queryParam := range q.Params {
		key := queryParam.Key()
		paramIndex, ok := tp.invertIndex[key]
		if !ok {
			if len(queryParam.Values) > 0 || queryParam.Reg != nil {
				return nil
			}
			continue
		}

		matchedIDs := tp.getMatchedIDs(&queryParam, paramIndex)

		hasPositiveConstraints := len(queryParam.Values) > 0 || queryParam.Reg != nil
		if hasPositiveConstraints && len(matchedIDs) == 0 {
			return nil
		}

		if len(matchedIDs) > 0 {
			if first {
				currentResult = matchedIDs
				first = false
			} else {
				currentResult = intersect(currentResult, matchedIDs)
			}
		}

		excludedIDs := tp.getExcludedIDs(&queryParam, paramIndex)
		if len(excludedIDs) > 0 {
			if first {
				initAllTraces()
			}
			currentResult = difference(currentResult, excludedIDs)
		}

		if len(currentResult) == 0 {
			return nil
		}
	}

	if first {
		initAllTraces()
	}

	return currentResult
}

func (tp *InMemoryTraceParams) getMatchedIDs(queryParam *query.QueryParam, paramIndex map[int32][]int32) []int32 {
	var matchedIDs []int32
	for _, val := range queryParam.Values {
		if e, ok := tp.encoding[val]; ok {
			if ids, ok := paramIndex[e]; ok {
				matchedIDs = append(matchedIDs, ids...)
			}
		}
	}
	if queryParam.Reg != nil {
		for e, ids := range paramIndex {
			if queryParam.Reg.MatchString(tp.rEncoding[e]) {
				matchedIDs = append(matchedIDs, ids...)
			}
		}
	}
	if len(matchedIDs) > 0 {
		sort.Slice(matchedIDs, func(i, j int) bool { return matchedIDs[i] < matchedIDs[j] })
		matchedIDs = deduplicate(matchedIDs)
	}
	return matchedIDs
}

func (tp *InMemoryTraceParams) getExcludedIDs(queryParam *query.QueryParam, paramIndex map[int32][]int32) []int32 {
	var excludedIDs []int32
	if len(queryParam.NegativeValues) == 0 && queryParam.NegativeReg == nil {
		return nil
	}
	for _, val := range queryParam.NegativeValues {
		if e, ok := tp.encoding[val]; ok {
			if ids, ok := paramIndex[e]; ok {
				excludedIDs = append(excludedIDs, ids...)
			}
		}
	}
	if queryParam.NegativeReg != nil {
		for e, ids := range paramIndex {
			if queryParam.NegativeReg.MatchString(tp.rEncoding[e]) {
				excludedIDs = append(excludedIDs, ids...)
			}
		}
	}
	if len(excludedIDs) > 0 {
		sort.Slice(excludedIDs, func(i, j int) bool { return excludedIDs[i] < excludedIDs[j] })
		excludedIDs = deduplicate(excludedIDs)
	}
	return excludedIDs
}

func buildInvertedIndex(paramCols map[string]int32, traceparams [][]int32) map[string]map[int32][]int32 {
	sklog.Infof("Building inverted index...")
	invertIndex := map[string]map[int32][]int32{}
	for p, c := range paramCols {
		invertIndex[p] = map[int32][]int32{}
		column := traceparams[c]
		for traceIdIdx, val := range column {
			if val >= 0 {
				invertIndex[p][val] = append(invertIndex[p][val], int32(traceIdIdx))
			}
		}
	}
	sklog.Infof("Inverted index built.")
	return invertIndex
}

func deduplicate(a []int32) []int32 {
	if len(a) <= 1 {
		return a
	}
	result := make([]int32, 0, len(a))
	result = append(result, a[0])
	for i := 1; i < len(a); i++ {
		if a[i] != a[i-1] {
			result = append(result, a[i])
		}
	}
	return result
}

func intersect(a, b []int32) []int32 {
	result := make([]int32, 0, min(len(a), len(b)))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] < b[j] {
			i++
		} else if a[i] > b[j] {
			j++
		} else {
			result = append(result, a[i])
			i++
			j++
		}
	}
	return result
}

func difference(a, b []int32) []int32 {
	result := make([]int32, 0, len(a))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i] < b[j] {
			result = append(result, a[i])
			i++
		} else if a[i] > b[j] {
			j++
		} else {
			i++
			j++
		}
	}
	for i < len(a) {
		result = append(result, a[i])
		i++
	}
	return result
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// ShowOnlyPublicTraces returns true if the instance is configured in showOnlyPublicTraces mode.
func (tp *InMemoryTraceParams) ShowOnlyPublicTraces() bool {
	return tp.showOnlyPublicTraces
}

// GetParamSet returns the pre-computed public parameters in O(1) time under a read lock.
func (tp *InMemoryTraceParams) GetParamSet() paramtools.ReadOnlyParamSet {
	tp.dataLock.RLock()
	defer tp.dataLock.RUnlock()
	return tp.publicParamSet
}

// buildParamSet compiles unique parameters from the encoded trace rows.
func buildParamSet(traceparams [][]int32, paramCols map[string]int32, rEncoding map[int32]string) paramtools.ParamSet {
	ret := paramtools.NewParamSet()
	if len(traceparams) == 0 {
		return ret
	}
	numTraces := len(traceparams[0])
	for key, colIndex := range paramCols {
		column := traceparams[colIndex]
		uniqueValues := map[int32]bool{}
		for i := 0; i < numTraces; i++ {
			val := column[i]
			if val >= 0 {
				uniqueValues[val] = true
			}
		}
		if len(uniqueValues) > 0 {
			valuesList := make([]string, 0, len(uniqueValues))
			for val := range uniqueValues {
				valuesList = append(valuesList, rEncoding[val])
			}
			ret[key] = valuesList
		}
	}
	ret.Normalize()
	return ret
}

// GetWasmCache returns the Wasm cache data for the latest tile.
func (tp *InMemoryTraceParams) GetWasmCache(ctx context.Context, ps paramtools.ReadOnlyParamSet) (*tracestore.WasmCacheData, error) {
	tp.dataLock.RLock()
	defer tp.dataLock.RUnlock()

	if len(tp.traceparams) == 0 {
		return nil, skerr.Fmt("No trace parameters in memory")
	}

	numTraces := len(tp.traceparams[0])
	if numTraces == 0 {
		return nil, skerr.Fmt("No traces in memory")
	}

	// 1. Compute commonParams
	commonParams := map[string]string{}
	for key, colIdx := range tp.paramCols {
		column := tp.traceparams[colIdx]
		if len(column) == 0 {
			continue
		}
		firstVal := column[0]
		if firstVal < 0 {
			continue // Missing in first trace, so not common
		}
		isCommon := true
		for i := 1; i < numTraces; i++ {
			if column[i] != firstVal {
				isCommon = false
				break
			}
		}
		if isCommon {
			commonParams[key] = tp.rEncoding[firstVal]
		}
	}

	// 2. Filter ParamSet to remove common keys
	filteredPs := paramtools.ParamSet{}
	for k, v := range ps {
		if _, ok := commonParams[k]; !ok {
			filteredPs[k] = v
		}
	}

	// 3. Build lookup and params
	var keys []string
	for k := range filteredPs {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// globalLookup is a 2D slice: globalLookup[colIdx][valIdx] -> WasmParam.Id
	// This avoids map lookups in the hot loop, making it as fast as a flat array.
	globalLookup := make([][]uint16, len(tp.paramCols))
	for i := range globalLookup {
		globalLookup[i] = make([]uint16, len(tp.encoding))
	}
	var idCounter uint16 = 1
	var wasmParams []tracestore.WasmParam

	for _, key := range keys {
		colIdx, ok := tp.paramCols[key]
		if !ok {
			continue
		}
		values := filteredPs[key]
		// Sort values for stability
		sortedValues := make([]string, len(values))
		copy(sortedValues, values)
		sort.Strings(sortedValues)

		for _, val := range sortedValues {
			id := idCounter
			idCounter++
			wasmParams = append(wasmParams, tracestore.WasmParam{Id: id, Key: key, Value: val})
			if e, ok := tp.encoding[val]; ok {
				globalLookup[colIdx][e] = id
			}
		}
	}

	stride := len(filteredPs)
	if stride%8 != 0 {
		stride = (stride/8 + 1) * 8
	}

	// 4. Encode traces
	tracesBinary := make([]byte, numTraces*stride*2)
	row := make([]uint16, stride)

	activeColIdxs := make([]int32, 0, len(keys))
	for _, key := range keys {
		if colIdx, ok := tp.paramCols[key]; ok {
			activeColIdxs = append(activeColIdxs, colIdx)
		}
	}

	for t := 0; t < numTraces; t++ {
		// Reset row
		for i := range row {
			row[i] = 0
		}

		i := 0
		for _, colIdx := range activeColIdxs {
			valIdx := tp.traceparams[colIdx][t]
			if valIdx >= 0 {
				id := globalLookup[colIdx][valIdx]
				if id > 0 {
					row[i] = id
					i++
				}
			}
		}

		// Write row to tracesBinary
		rowOffset := t * stride * 2
		for rIdx, v := range row {
			tracesBinary[rowOffset+rIdx*2] = byte(v)
			tracesBinary[rowOffset+rIdx*2+1] = byte(v >> 8)
		}
	}

	// 5. Marshal meta and params
	version := fmt.Sprintf("%d", time.Now().Unix())
	meta := struct {
		Stride       int               `json:"stride"`
		Count        int               `json:"count"`
		Version      string            `json:"version"`
		CommonParams map[string]string `json:"commonParams"`
	}{
		Stride:       stride,
		Count:        numTraces,
		Version:      version,
		CommonParams: commonParams,
	}

	metaBytes, err := json.Marshal(meta)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	paramsBytes, err := json.Marshal(wasmParams)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &tracestore.WasmCacheData{
		Meta:   metaBytes,
		Params: paramsBytes,
		Traces: tracesBinary,
	}, nil
}
