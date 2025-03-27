package schema

import "go.skia.org/infra/perf/go/types"

// TraceValuesSchema describes the SQL schema of the TraceValues table.
type TraceValuesSchema struct {
	TraceID           []byte   `sql:"trace_id BYTES"`
	CommitNumber      int64    `sql:"commit_number INT"`
	Value             float32  `sql:"val REAL"`
	SourceFileID      int64    `sql:"source_file_id INT"`
	primaryKey        struct{} `sql:"PRIMARY KEY (trace_id, commit_number)"`
	bySourceFileIndex struct{} `sql:"INDEX by_source_file_id (source_file_id, trace_id)"`
}

type TraceValues2Schema struct {
	TraceID           []byte   `sql:"trace_id BYTES"`
	CommitNumber      int64    `sql:"commit_number INT"`
	Value             float32  `sql:"val REAL"`
	SourceFileID      int64    `sql:"source_file_id INT"`
	Benchmark         string   `sql:"benchmark STRING"`
	Bot               string   `sql:"bot STRING"`
	Test              string   `sql:"test STRING"`
	Subtest_1         string   `sql:"subtest_1 STRING"`
	Subtest_2         string   `sql:"subtest_2 STRING"`
	Subtest_3         string   `sql:"subtest_3 STRING"`
	primaryKey        struct{} `sql:"PRIMARY KEY (trace_id, commit_number)"`
	bySourceFileIndex struct{} `sql:"INDEX by_trace_id_tv2 (trace_id, benchmark, bot, test, subtest_1, subtest_2, subtest_3)"`
}

type SourceFilesSchema struct {
	ID         int64    `sql:"source_file_id INT PRIMARY KEY DEFAULT unique_rowid()"`
	SourceFile string   `sql:"source_file STRING UNIQUE NOT NULL"`
	index      struct{} `sql:"INDEX by_source_file (source_file, source_file_id)"`
}

type ParamSetsSchema struct {
	TileNumber        types.TileNumber `sql:"tile_number INT"`
	ParamKey          string           `sql:"param_key STRING"`
	ParamValue        string           `sql:"param_value STRING"`
	primaryKey        struct{}         `sql:"PRIMARY KEY (tile_number, param_key, param_value)"`
	byTileNumberIndex struct{}         `sql:"INDEX by_tile_number (tile_number DESC)"`
}

type PostingsSchema struct {
	TileNumber      types.TileNumber `sql:"tile_number INT"`
	KeyValue        string           `sql:"key_value STRING NOT NULL"`
	TraceID         []byte           `sql:"trace_id BYTES"`
	primaryKey      struct{}         `sql:"PRIMARY KEY (tile_number, key_value, trace_id)"`
	byTraceIDIndex  struct{}         `sql:"INDEX by_trace_id (tile_number, trace_id, key_value)"`
	byTraceIDIndex2 struct{}         `sql:"INDEX by_trace_id2 (tile_number, trace_id)"`
	byKeyValueIndex struct{}         `sql:"INDEX by_key_value (tile_number, key_value)"`
}

type MetadataSchema struct {
	SourceFileId int64             `sql:"source_file_id INT PRIMARY KEY"`
	Links        map[string]string `sql:"links JSONB"`
}
