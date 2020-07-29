-- This table is used to store trace names. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS TraceNames  (
	trace_id BYTES PRIMARY KEY,       -- md5(trace_name)
	params JSONB NOT NULL, -- The params that make up the trace_id, {"arch=x86", "config=8888"}.
    INVERTED INDEX (params)
);

-- This table is used to store trace values. See go/tracestore/sqltracestore.
CREATE TABLE IF NOT EXISTS TraceValues2 (
	trace_id BYTES,                     -- Id of the trace name from TraceNames.
	commit_number INT,                   -- A types.CommitNumber.
	val REAL,                            -- The floating point measurement.
	source_file_id INT,                  -- Id of the source filename, from SourceFiles.
	PRIMARY KEY (trace_id, commit_number)
);
