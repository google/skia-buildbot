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