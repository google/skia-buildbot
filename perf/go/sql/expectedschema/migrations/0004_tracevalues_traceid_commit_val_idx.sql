-- Rollback SQL (For manual reference):
-- DROP INDEX IF EXISTS by_traceid_commit_number_desc_include_val;

CREATE INDEX IF NOT EXISTS by_traceid_commit_number_desc_include_val ON TraceValues (trace_id, commit_number DESC) INCLUDE (val);
