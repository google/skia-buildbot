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
	byKeyValueIndex struct{}         `sql:"INDEX by_key_value (tile_number, key_value)"`
}
