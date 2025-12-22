package sqltracestore

import (
	"bytes"
	"context"
	"encoding/hex"
	"strings"
	"text/template"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/perf/go/tracestore"
)

const (
	writeTraceParams int = iota
	readTraceParams
)

const traceParamInsertChunkSize = 100
const traceParamInsertParallelPoolSize = 5

var stmts = map[int]string{

	readTraceParams: `SELECT trace_id, params
        FROM TraceParams
   			WHERE
      	trace_id = ANY($1);
		`,
	writeTraceParams: `INSERT INTO
        TraceParams (trace_id, params)
        VALUES
            {{ range $index, $trace_id  :=  .MD5HexTraceIDs -}}
                {{ if $index }},{{end}}
                ( '{{ $trace_id }}', {{ print "$"}}{{ increment $index }} )
            {{ end }}
        ON CONFLICT (trace_id) DO NOTHING
        `,
}

// traceParamsContext provides a context struct to execute the query template.
type traceParamsContext struct {
	MD5HexTraceIDs []string
}

// SQLTraceParamStore implements tracestore.TraceParamStore.
type SQLTraceParamStore struct {
	// db is the SQL database instance.
	db pool.Pool
}

// NewTraceParamStore returns a new instance of the SQLTraceParamStore struct.
func NewTraceParamStore(db pool.Pool) *SQLTraceParamStore {
	return &SQLTraceParamStore{
		db: db,
	}
}

// ReadParams reads the parameters for the given set of traceIds.
func (s *SQLTraceParamStore) ReadParams(ctx context.Context, traceIds []string) (map[string]paramtools.Params, error) {
	if len(traceIds) == 0 {
		return nil, nil
	}

	traceIdsAsBytes, err := convertTraceIDsToBytes(traceIds)
	if err != nil {
		return nil, err
	}

	rows, err := s.db.Query(ctx, stmts[readTraceParams], traceIdsAsBytes)
	if err != nil {
		return nil, err
	}

	defer rows.Close()
	traceParams := map[string]paramtools.Params{}
	for rows.Next() {
		var trace_id []byte
		var params paramtools.Params
		if err := rows.Scan(&trace_id, &params); err != nil {
			return nil, err
		}

		traceIdString := traceIDForSQLFromTraceIDAsBytes(trace_id)
		traceParams[string(traceIdString)] = params
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return traceParams, nil
}

// WriteTraceParams writes the given trace params into the table. The key for the
// traceParams is the traceId (hex-encoded form of the md5 hash of the trace name)
// and the value is the corresponding params.
func (s *SQLTraceParamStore) WriteTraceParams(ctx context.Context, traceParams map[string]paramtools.Params) error {
	// If the traceParams list is small enough, write it directly.
	if len(traceParams) <= traceParamInsertChunkSize {
		return s.writeTraceParamsChunk(ctx, traceParams)
	}

	keys := []string{}
	for key := range traceParams {
		keys = append(keys, key)
	}

	// Since this is a big list, we will write it in chunks in parallel.
	err := util.ChunkIterParallelPool(ctx, len(traceParams), traceParamInsertChunkSize, traceParamInsertParallelPoolSize, func(ctx context.Context, startIdx, endIdx int) error {
		chunkKeys := keys[startIdx:endIdx]
		filteredTraceParams := map[string]paramtools.Params{}
		for _, key := range chunkKeys {
			filteredTraceParams[key] = traceParams[key]
		}
		return s.writeTraceParamsChunk(ctx, filteredTraceParams)
	})

	return err
}

// writeTraceParamsChunk writes the given chunk of traceParams to the traceparams table.
func (s *SQLTraceParamStore) writeTraceParamsChunk(ctx context.Context, traceParams map[string]paramtools.Params) error {
	paramList := make([]interface{}, len(traceParams))
	i := 0
	insertContext := traceParamsContext{}
	for traceId, params := range traceParams {
		insertContext.MD5HexTraceIDs = append(insertContext.MD5HexTraceIDs, traceId)
		paramList[i] = params
		i++
	}
	// We cannot use the Params inside the template since it converts
	// the map object into a string. Therefore in the template expansion,
	// we add place holders ($1, $2, etc) for the params field in the VALUES list.
	// The $index var starts at 0 while placeholders start at 1, hence we add an
	// "increment" func to be used inside the template.
	writeStmt := stmts[writeTraceParams]
	funcMap := template.FuncMap{
		"increment": func(i int) int {
			return i + 1
		},
	}
	sqltemplate, err := template.New("").Funcs(funcMap).Parse(writeStmt)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Expand the template for the SQL.
	var b bytes.Buffer
	if err := sqltemplate.Execute(&b, insertContext); err != nil {
		return skerr.Wrapf(err, "failed to expand writeTraceParams template")
	}
	sql := b.String()
	if _, err := s.db.Exec(ctx, sql, paramList...); err != nil {
		return skerr.Wrapf(err, "Executing: %q", b.String())
	}
	return nil
}

func traceIDAsBytesFromtraceIDAsString(id string) ([]byte, error) {
	s := strings.TrimPrefix(id, `\x`)
	b, err := hex.DecodeString(s)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to decode trace id: %q", s)
	}
	return b, nil
}

func convertTraceIDsToBytes(traceIds []string) ([][]byte, error) {
	traceIdsAsBytes := make([][]byte, len(traceIds))
	for i, id := range traceIds {
		bytesId, err := traceIDAsBytesFromtraceIDAsString(id)
		if err != nil {
			return nil, err
		}
		traceIdsAsBytes[i] = bytesId
	}
	return traceIdsAsBytes, nil
}

var _ tracestore.TraceParamStore = (*SQLTraceParamStore)(nil)
