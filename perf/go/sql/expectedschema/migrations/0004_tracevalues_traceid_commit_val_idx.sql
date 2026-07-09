-- Rollback SQL (For manual reference):
-- DROP INDEX by_traceid_commit_number_desc_include_val;

CREATE INDEX by_traceid_commit_number_desc_include_val ON TraceValues (trace_id, commit_number DESC) INCLUDE (val);
