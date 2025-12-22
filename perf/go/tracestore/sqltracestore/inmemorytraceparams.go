package sqltracestore

import (
	"context"
	"encoding/hex"
	"fmt"
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
	"go.skia.org/infra/perf/go/types"
)

type InMemoryTraceParams struct {
	db                       pool.Pool
	refreshIntervalInSeconds float32

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

	// Lock around "in-memory" data. Querying takes RLock (non-exclusive),
	// refreshing takes Lock (exclusive).
	dataLock sync.RWMutex
}

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
	traceparams, encoding, rEncoding, err := tp.readAllTraceParams(ctx, traceparams, paramCols)
	if err != nil {
		return skerr.Wrap(err)
	}

	tp.dataLock.Lock()
	defer tp.dataLock.Unlock()
	tp.traceparams = traceparams
	tp.paramCols = paramCols
	tp.colParams = colParams
	tp.encoding = encoding
	tp.rEncoding = rEncoding
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
func (tp *InMemoryTraceParams) readAllTraceParams(ctx context.Context, traceparams [][]int32, paramCols map[string]int32) ([][]int32, map[string]int32, map[int32]string, error) {
	var wg sync.WaitGroup
	var traceParamReadWg sync.WaitGroup
	totalRows := int64(0)
	errors := make(chan error, traceParamsReadPartitionCount)
	traceParamsChan := make(chan TraceParam, 10000)
	encoding := map[string]int32{}
	rEncoding := map[int32]string{}

	var traceIdCount int = 0
	var eCount int32 = 0

	totalRowMutex := sync.Mutex{}

	// Start a goroutine that will read the traceparam entries being published by the workers.
	traceParamReadWg.Add(1)
	go func() {
		defer traceParamReadWg.Done()
		for traceParam := range traceParamsChan {
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
		return nil, nil, nil, err // Return the first error encountered
	}

	sklog.Infof("TraceParams Partitioned Read complete. Total estimated rows processed: %d", totalRows)
	return traceparams, encoding, rEncoding, nil
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
func NewInMemoryTraceParams(ctx context.Context, db pool.Pool, refreshIntervalInSeconds float32) (*InMemoryTraceParams, error) {
	ctx, span := trace.StartSpan(ctx, "InMemoryTraceParams.NewInMemoryTraceParams")
	defer span.End()

	ret := InMemoryTraceParams{
		db:                       db,
		refreshIntervalInSeconds: refreshIntervalInSeconds,
	}
	err := ret.startRefresher(ctx)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	return &ret, nil
}

// Query for paramsets in (in-memory version of) traceparams table matching query input
func (tp *InMemoryTraceParams) QueryTraceIDs(ctx context.Context, tileNumber types.TileNumber,
	q *query.Query, outParams chan paramtools.Params) {
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
					values := queryParam.Values
					colIndex := tp.paramCols[key]
					column := tp.traceparams[colIndex]
					// Encode param values in query up-front to avoid doing it
					// repeatedly looping over traceids
					var eParamValues []int32 = []int32{}
					for _, paramValue := range values {
						e, ok := tp.encoding[paramValue]
						if ok {
							eParamValues = append(eParamValues, e)
						}
					}

					// Iterate over traceids and keep them only if they match the query
					nextTraceidCount := 0
					for i := range traceidCount {
						traceIdIdx := traceids[i]
						if queryParam.IsRegex {
							// Keep traceid if the unencoded string matches the regex
							if queryParam.Reg.String() != "" &&
								queryParam.Reg.MatchString(tp.rEncoding[column[traceIdIdx]]) {
								traceids[nextTraceidCount] = traceIdIdx
								nextTraceidCount++
							}
						} else if queryParam.IsNegative {
							// Keep traceid if it does not match encoded param value
							for _, eParamValue := range eParamValues {
								if column[traceIdIdx] != eParamValue {
									traceids[nextTraceidCount] = traceIdIdx
									nextTraceidCount++
								}
							}

						} else {
							// Keep traceid if it matches any encoded param value
							for _, eParamValue := range eParamValues {
								if column[traceIdIdx] == eParamValue {
									traceids[nextTraceidCount] = traceIdIdx
									nextTraceidCount++
									break
								}
							}
						}
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
