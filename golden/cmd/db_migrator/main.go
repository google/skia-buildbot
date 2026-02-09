package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgtype"
	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/sql/schema/spanner"
)

func main() {
	cdbURL := flag.String("cdb_url", "", "CockroachDB connection URL")
	spannerURL := flag.String("spanner_url", "", "Spanner connection URL")
	batchSize := flag.Int("batch_size", 1000, "Batch size for copying rows")
	continuous := flag.Bool("continuous", false, "If true, keep running until caught up and then poll for more")

	flag.Parse()

	if *cdbURL == "" || *spannerURL == "" {
		sklog.Fatalf("Both cdb_url and spanner_url must be provided")
	}

	ctx := context.Background()

	cdbPool, err := pgxpool.Connect(ctx, *cdbURL)
	if err != nil {
		sklog.Fatalf("Failed to connect to CockroachDB: %s", err)
	}
	defer cdbPool.Close()

	spannerPool, err := pgxpool.Connect(ctx, *spannerURL)
	if err != nil {
		sklog.Fatalf("Failed to connect to Spanner: %s", err)
	}
	defer spannerPool.Close()

	if err := initSpannerSchema(ctx, spannerPool); err != nil {
		sklog.Fatalf("Failed to initialize Spanner schema: %s", err)
	}

	// List of the tables in Gold. Refer to //golden/go/sql/schema for more details.
	tables := []string{
		"Changelists",
		"Patchsets",
		"Tryjobs",
		"ExpectationRecords",
		"ExpectationDeltas",
		"Expectations",
		"Groupings",
		"Options",
		"SourceFiles",
		"Traces",
		"CommitsWithData",
		"GitCommits",
		"MetadataCommits",
		"TrackingCommits",
		"TraceValues",
		"ValuesAtHead",
		"PrimaryBranchParams",
		"TiledTraceDigests",
		"DiffMetrics",
		"IgnoreRules",
		"ProblemImages",
		"PrimaryBranchDiffCalculationWork",
		"SecondaryBranchDiffCalculationWork",
		"SecondaryBranchExpectations",
		"SecondaryBranchParams",
		"SecondaryBranchValues",
	}

	for {
		workDone := false
		for _, tableName := range tables {
			done, err := migrateTable(ctx, cdbPool, spannerPool, tableName, *batchSize)
			if err != nil {
				sklog.Errorf("Error migrating table %s: %s", tableName, err)
				continue
			}
			if !done {
				workDone = true
			}
		}

		if !*continuous && !workDone {
			break
		}
		if !workDone {
			sklog.Info("Caught up, sleeping...")
			time.Sleep(1 * time.Minute)
		}
	}

	sklog.Info("Migration completed")
}

var tableColumns = map[string][]string{
	"Changelists":                        {"changelist_id", "system", "status", "owner_email", "subject", "last_ingested_data"},
	"Patchsets":                          {"patchset_id", "system", "changelist_id", "ps_order", "git_hash", "commented_on_cl", "created_ts"},
	"Tryjobs":                            {"tryjob_id", "system", "changelist_id", "patchset_id", "display_name", "last_ingested_data"},
	"ExpectationRecords":                 {"expectation_record_id", "branch_name", "user_name", "triage_time", "num_changes"},
	"ExpectationDeltas":                  {"expectation_record_id", "grouping_id", "digest", "label_before", "label_after"},
	"Expectations":                       {"grouping_id", "digest", "label", "expectation_record_id"},
	"Groupings":                          {"grouping_id", "keys"},
	"Options":                            {"options_id", "keys"},
	"SourceFiles":                        {"source_file_id", "source_file", "last_ingested"},
	"Traces":                             {"trace_id", "grouping_id", "keys", "matches_any_ignore_rule"},
	"CommitsWithData":                    {"commit_id", "tile_id"},
	"GitCommits":                         {"git_hash", "commit_id", "commit_time", "author_email", "subject"},
	"MetadataCommits":                    {"commit_id", "commit_metadata"},
	"TrackingCommits":                    {"repo", "last_git_hash"},
	"TraceValues":                        {"shard", "trace_id", "commit_id", "digest", "grouping_id", "options_id", "source_file_id"},
	"ValuesAtHead":                       {"trace_id", "most_recent_commit_id", "digest", "options_id", "grouping_id", "keys", "matches_any_ignore_rule"},
	"PrimaryBranchParams":                {"tile_id", "key", "value"},
	"TiledTraceDigests":                  {"trace_id", "tile_id", "digest", "grouping_id"},
	"DiffMetrics":                        {"left_digest", "right_digest", "num_pixels_diff", "percent_pixels_diff", "max_rgba_diffs", "max_channel_diff", "combined_metric", "dimensions_differ", "ts"},
	"IgnoreRules":                        {"ignore_rule_id", "creator_email", "updated_email", "expires", "note", "query"},
	"ProblemImages":                      {"digest", "num_errors", "latest_error", "error_ts"},
	"PrimaryBranchDiffCalculationWork":   {"grouping_id", "last_calculated_ts", "calculation_lease_ends"},
	"SecondaryBranchDiffCalculationWork": {"branch_name", "grouping_id", "last_updated_ts", "digests", "last_calculated_ts", "calculation_lease_ends"},
	"SecondaryBranchExpectations":        {"branch_name", "grouping_id", "digest", "label", "expectation_record_id"},
	"SecondaryBranchParams":              {"branch_name", "version_name", "key", "value"},
	"SecondaryBranchValues":              {"branch_name", "version_name", "secondary_branch_trace_id", "digest", "grouping_id", "options_id", "source_file_id", "tryjob_id"},
}

var tablePrimaryKeys = map[string][]string{
	"Changelists":                        {"changelist_id"},
	"Patchsets":                          {"patchset_id"},
	"Tryjobs":                            {"tryjob_id"},
	"ExpectationRecords":                 {"expectation_record_id"},
	"ExpectationDeltas":                  {"expectation_record_id", "grouping_id", "digest"},
	"Expectations":                       {"grouping_id", "digest"},
	"Groupings":                          {"grouping_id"},
	"Options":                            {"options_id"},
	"SourceFiles":                        {"source_file_id"},
	"Traces":                             {"trace_id"},
	"CommitsWithData":                    {"commit_id"},
	"GitCommits":                         {"git_hash"},
	"MetadataCommits":                    {"commit_id"},
	"TrackingCommits":                    {"repo"},
	"TraceValues":                        {"shard", "trace_id", "commit_id"},
	"ValuesAtHead":                       {"trace_id"},
	"PrimaryBranchParams":                {"tile_id", "key", "value"},
	"TiledTraceDigests":                  {"trace_id", "tile_id", "digest"},
	"DiffMetrics":                        {"left_digest", "right_digest"},
	"IgnoreRules":                        {"ignore_rule_id"},
	"ProblemImages":                      {"digest"},
	"PrimaryBranchDiffCalculationWork":   {"grouping_id"},
	"SecondaryBranchDiffCalculationWork": {"branch_name", "grouping_id"},
	"SecondaryBranchExpectations":        {"branch_name", "grouping_id", "digest"},
	"SecondaryBranchParams":              {"branch_name", "version_name", "key", "value"},
	"SecondaryBranchValues":              {"branch_name", "version_name", "secondary_branch_trace_id", "source_file_id"},
}

var uuidColumns = map[string]bool{
	"expectation_record_id": true,
	"ignore_rule_id":        true,
	"expectation_id":        true,
}

func initSpannerSchema(ctx context.Context, db *pgxpool.Pool) error {
	sklog.Info("Initializing Spanner schema...")
	if _, err := db.Exec(ctx, spanner.Schema); err != nil {
		return err
	}
	_, err := db.Exec(ctx, `CREATE TABLE IF NOT EXISTS migration_progress (
		table_name TEXT PRIMARY KEY,
		last_processed_values JSONB,
		updated_at TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP
	)`)
	return err
}

func migrateTable(ctx context.Context, src, dst *pgxpool.Pool, tableName string, batchSize int) (bool, error) {
	sklog.Infof("Migrating table %s", tableName)
	cols := tableColumns[tableName]
	pkCols := tablePrimaryKeys[tableName]
	if len(cols) == 0 || len(pkCols) == 0 {
		return true, skerr.Fmt("Unknown table %s", tableName)
	}

	orderByCols := pkCols

	lastValues, err := getProgress(ctx, dst, tableName)
	if err != nil {
		return false, err
	}

	query := buildQuery(tableName, orderByCols, lastValues, batchSize)
	rows, err := src.Query(ctx, query, lastValues...)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var batch [][]interface{}
	for rows.Next() {
		values, err := rows.Values()
		if err != nil {
			return false, err
		}
		sanitizeRow(cols, values)
		batch = append(batch, values)
	}

	if len(batch) == 0 {
		return true, nil
	}

	// Process the batch
	if err := writeBatch(ctx, dst, tableName, cols, pkCols, batch); err != nil {
		return false, err
	}

	// Get progress values from the last row
	lastRow := batch[len(batch)-1]
	lastRowValues := extractProgressValues(lastRow, cols, orderByCols)

	if err := saveProgress(ctx, dst, tableName, lastRowValues); err != nil {
		return false, err
	}

	sklog.Infof("Migrated batch of %d rows for %s", len(batch), tableName)
	return len(batch) < batchSize, nil
}

func sanitizeRow(cols []string, values []interface{}) {
	for i, colName := range cols {
		val := values[i]
		if val == nil {
			continue
		}

		// Handle specialized types from pgx or UUIDs.
		switch v := val.(type) {
		case pgtype.Int2Array:
			var res []int16
			if err := v.AssignTo(&res); err == nil {
				res64 := make([]int64, len(res))
				for j, iv := range res {
					res64[j] = int64(iv)
				}
				values[i] = res64
				continue
			}
		case pgtype.Int4Array:
			var res []int32
			if err := v.AssignTo(&res); err == nil {
				res64 := make([]int64, len(res))
				for j, iv := range res {
					res64[j] = int64(iv)
				}
				values[i] = res64
				continue
			}
		case pgtype.Int8Array:
			var res []int64
			if err := v.AssignTo(&res); err == nil {
				values[i] = res
				continue
			}
		case uuid.UUID:
			values[i] = v.String()
			continue
		case [16]byte:
			if uuidColumns[colName] {
				if u, err := uuid.FromBytes(v[:]); err == nil {
					values[i] = u.String()
					continue
				}
			}
			// If not a UUID column, treat as BYTEA (e.g. MD5 hash) and fall through.
		}

		// Handle generic arrays/slices and basic integer/float types.
		rv := reflect.ValueOf(values[i])
		kind := rv.Kind()

		if kind == reflect.Array || kind == reflect.Slice {
			elemKind := rv.Type().Elem().Kind()
			// Handle integer arrays/slices (excluding byte/uint8 which is BYTEA)
			if (elemKind >= reflect.Int && elemKind <= reflect.Int64) ||
				(elemKind >= reflect.Uint && elemKind <= reflect.Uint64 && elemKind != reflect.Uint8) {
				res := make([]int64, rv.Len())
				for j := 0; j < rv.Len(); j++ {
					res[j] = toInt64(rv.Index(j).Interface())
				}
				values[i] = res
				continue
			}
		}

		if (kind >= reflect.Int && kind <= reflect.Int64) ||
			(kind >= reflect.Uint && kind <= reflect.Uint64 && kind != reflect.Uint8) {
			values[i] = toInt64(values[i])
		} else if kind == reflect.Float32 || kind == reflect.Float64 {
			values[i] = rv.Float()
		}
	}
}

func toInt64(v interface{}) int64 {
	switch val := v.(type) {
	case int8:
		return int64(val)
	case int16:
		return int64(val)
	case int32:
		return int64(val)
	case int64:
		return val
	case int:
		return int64(val)
	case uint8:
		return int64(val)
	case uint16:
		return int64(val)
	case uint32:
		return int64(val)
	case uint64:
		return int64(val)
	default:
		return 0
	}
}

func buildQuery(tableName string, orderByCols []string, lastValues []interface{}, batchSize int) string {
	var sb strings.Builder
	sb.WriteString("SELECT ")
	cols := tableColumns[tableName]
	sb.WriteString(strings.Join(cols, ", "))

	sb.WriteString(" FROM ")
	sb.WriteString(strings.ToLower(tableName))
	if len(lastValues) > 0 {
		sb.WriteString(" WHERE (")
		sb.WriteString(strings.Join(orderByCols, ", "))
		sb.WriteString(") > (")
		for i := range lastValues {
			if i > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("$%d", i+1))
		}
		sb.WriteString(")")
	}
	sb.WriteString(" ORDER BY ")
	sb.WriteString(strings.Join(orderByCols, ", "))
	sb.WriteString(fmt.Sprintf(" LIMIT %d", batchSize))
	return sb.String()
}

func extractProgressValues(row []interface{}, cols []string, orderByCols []string) []interface{} {
	var res []interface{}
	for _, col := range orderByCols {
		found := false
		for i, name := range cols {
			if strings.EqualFold(name, col) {
				res = append(res, row[i])
				found = true
				break
			}
		}
		if !found {
			panic(fmt.Sprintf("Column %s not found in selected columns for table", col))
		}
	}
	return res
}

type typedValue struct {
	Type  string      `json:"t"`
	Value interface{} `json:"v"`
}

func getProgress(ctx context.Context, db *pgxpool.Pool, tableName string) ([]interface{}, error) {
	var data []byte
	err := db.QueryRow(ctx, "SELECT last_processed_values FROM migration_progress WHERE table_name = $1", tableName).Scan(&data)
	if err == pgx.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var rawItems []interface{}
	if err := json.Unmarshal(data, &rawItems); err != nil {
		return nil, err
	}
	values := make([]interface{}, len(rawItems))
	for i, item := range rawItems {
		m, ok := item.(map[string]interface{})
		if !ok {
			// Old format: just use the value as is.
			values[i] = item
			continue
		}

		// New format: extract based on type hint.
		typeName, _ := m["t"].(string)
		val := m["v"]
		switch typeName {
		case "bytes":
			if s, ok := val.(string); ok {
				b, err := base64.StdEncoding.DecodeString(s)
				if err != nil {
					return nil, skerr.Wrapf(err, "decoding base64 bytes for %s", tableName)
				}
				values[i] = b
			}
		case "int64":
			if f, ok := val.(float64); ok {
				values[i] = int64(f)
			}
		case "time":
			if s, ok := val.(string); ok {
				t, err := time.Parse(time.RFC3339Nano, s)
				if err != nil {
					return nil, skerr.Wrapf(err, "parsing time for %s", tableName)
				}
				values[i] = t
			}
		default:
			values[i] = val
		}
	}
	return values, nil
}

func saveProgress(ctx context.Context, db *pgxpool.Pool, tableName string, values []interface{}) error {
	typedValues := make([]typedValue, len(values))
	for i, v := range values {
		switch t := v.(type) {
		case []byte:
			typedValues[i] = typedValue{Type: "bytes", Value: base64.StdEncoding.EncodeToString(t)}
		case [16]byte:
			typedValues[i] = typedValue{Type: "bytes", Value: base64.StdEncoding.EncodeToString(t[:])}
		case int64:
			typedValues[i] = typedValue{Type: "int64", Value: t}
		case int:
			typedValues[i] = typedValue{Type: "int64", Value: int64(t)}
		case string:
			typedValues[i] = typedValue{Type: "string", Value: t}
		case time.Time:
			typedValues[i] = typedValue{Type: "time", Value: t.Format(time.RFC3339Nano)}
		default:
			typedValues[i] = typedValue{Type: "unknown", Value: v}
		}
	}
	data, err := json.Marshal(typedValues)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO migration_progress (table_name, last_processed_values, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (table_name) DO UPDATE SET last_processed_values = EXCLUDED.last_processed_values, updated_at = EXCLUDED.updated_at`,
		tableName, data)
	return err
}

func writeBatch(ctx context.Context, db *pgxpool.Pool, tableName string, colNames []string, pkCols []string, batch [][]interface{}) error {
	if len(batch) == 0 {
		return nil
	}

	numCols := len(colNames)
	vp := "("
	for i := 0; i < numCols; i++ {
		if i > 0 {
			vp += ", "
		}
		vp += fmt.Sprintf("$%d", i+1)
	}
	vp += ")"

	// We use INSERT ... ON CONFLICT DO NOTHING to be idempotent.
	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES %s ON CONFLICT (%s) DO NOTHING",
		strings.ToLower(tableName),
		strings.Join(colNames, ", "),
		vp,
		strings.Join(pkCols, ", "))

	for _, values := range batch {
		if _, err := db.Exec(ctx, query, values...); err != nil {
			return skerr.Wrapf(err, "writing row to %s", tableName)
		}
	}

	return nil
}
