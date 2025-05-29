package sqltracestore

import (
	"context"
	"sync"
	"time"

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

func (tp *InMemoryTraceParams) refresh(ctx context.Context) error {
	ctx, span := trace.StartSpan(ctx, "InMemoryTraceParams.refresh")
	defer span.End()
	traceparams := [][]int32{}
	paramCols := map[string]int32{}
	colParams := map[int32]string{}
	encoding := map[string]int32{}
	rEncoding := map[int32]string{}

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
		return skerr.Wrap(err)
	}
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
	rows, err := tp.db.Query(ctx, "SELECT trace_id, params FROM traceparams;")
	if err != nil {
		return skerr.Wrap(err)
	}
	defer rows.Close()
	var traceIdCount int = 0
	var eCount int32 = 0
	for rows.Next() {
		var trace_id []byte
		var params map[string]interface{}
		err = rows.Scan(&trace_id, &params)
		if err != nil {
			sklog.Fatal(err)
		}
		// Try to read the string for each column
		for param, paramCol := range paramCols {
			var valueE int32 = -1
			value, ok := params[param]
			if ok {
				valueString := value.(string)
				valueE, ok = encoding[valueString]
				if !ok {
					// If string hasn't been encoded yet then map param values
					// to number encoding, and back
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

	tp.dataLock.Lock()
	defer tp.dataLock.Unlock()
	tp.traceparams = traceparams
	tp.paramCols = paramCols
	tp.colParams = colParams
	tp.encoding = encoding
	tp.rEncoding = rEncoding
	return nil
}

func (tp *InMemoryTraceParams) startRefresher(ctx context.Context) error {
	// Initialize
	err := tp.refresh(ctx)
	if err != nil {
		return err
	}

	// Update the cache periodically.
	go func() {
		// Periodically run it based on the specified duration.
		refreshDuration := time.Second * time.Duration(tp.refreshIntervalInSeconds)
		for range time.Tick(refreshDuration) {
			err := tp.refresh(ctx)
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
			sklog.Fatal(err)
		}
	}()
}
