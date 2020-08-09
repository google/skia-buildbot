-- This table is used to store trace names. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS TraceNames (
	-- md5(trace_name)
	trace_id BYTES PRIMARY KEY,
	-- The params that make up the trace_id, {"arch=x86", "config=8888"}.
	params JSONB NOT NULL,
	INVERTED INDEX (params)
);

-- This table is used to store trace values. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS TraceValues2 (
	-- Id of the trace name from TraceNames.
	trace_id BYTES,
	-- A types.CommitNumber.
	commit_number INT,
	-- The floating point measurement.
	val REAL,
	-- Id of the source filename, from SourceFiles.
	source_file_id INT,
	PRIMARY KEY (trace_id, commit_number)
);

CREATE TABLE IF NOT EXISTS Tiles (
	-- Id of the trace name from TraceNames.
	trace_id BYTES,
	-- The number of the tile that the trace_id appears in.
	tile_number INT,
	PRIMARY KEY (trace_id, tile_number) -- TODO(jcgregorio) May need reverse key.
);

CREATE TABLE IF NOT EXISTS ParamSets (
	tile_number INT,
	param_key STRING,
	param_value STRING,
	PRIMARY KEY (tile_number, param_key, param_value),
	INDEX (tile_number DESC),
);