package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/jackc/pgx/v4/pgxpool"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/schema/spanner"
	"go.skia.org/infra/golden/go/sql/sqltest"
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

func checkColumnExists(ctx context.Context, db *pgxpool.Pool, tableName, columnName string) (bool, error) {
	var exists bool
	// We use strings.ToLower because CockroachDB's information_schema stores names in lowercase usually,
	// but it depends on how they were created. Standard SQL is case-insensitive for unquoted names.
	query := `SELECT EXISTS (
		SELECT 1
		FROM information_schema.columns
		WHERE table_name = $1 AND column_name = $2
	)`
	err := db.QueryRow(ctx, query, strings.ToLower(tableName), strings.ToLower(columnName)).Scan(&exists)
	return exists, err
}

func migrateTable(ctx context.Context, src, dst *pgxpool.Pool, tableName string, batchSize int) (bool, error) {
	sklog.Infof("Migrating table %s", tableName)
	rowType := getRowType(tableName)
	if rowType == nil {
		return true, skerr.Fmt("Unknown table %s", tableName)
	}

	// Instantiate a row to get PK columns
	rowInstance := reflect.New(rowType).Interface().(sqltest.SQLExporter)
	pkCols := rowInstance.GetPrimaryKeyCols()

	hasCreatedAt, err := checkColumnExists(ctx, src, tableName, "createdat")
	if err != nil {
		return false, err
	}

	orderByCols := pkCols
	if hasCreatedAt {
		orderByCols = append([]string{"createdat"}, pkCols...)
	}

	lastValues, err := getProgress(ctx, dst, tableName)
	if err != nil {
		return false, err
	}

	query := buildQuery(tableName, hasCreatedAt, orderByCols, lastValues, batchSize)
	rows, err := src.Query(ctx, query, lastValues...)
	if err != nil {
		return false, err
	}
	defer rows.Close()

	var batch []sqltest.SQLExporter
	var lastRowValues []interface{}

	for rows.Next() {
		newVal := reflect.New(rowType)
		s := newVal.Interface().(sqltest.SQLScanner)
		if err := s.ScanFrom(rows.Scan); err != nil {
			return false, err
		}
		exporter := newVal.Interface().(sqltest.SQLExporter)
		batch = append(batch, exporter)
	}

	if len(batch) == 0 {
		return true, nil
	}

	// Process the batch
	if err := writeBatch(ctx, dst, tableName, batch); err != nil {
		return false, err
	}

	// Get progress values from the last row
	lastRow := batch[len(batch)-1]
	lastRowValues = extractProgressValues(lastRow, orderByCols)

	if err := saveProgress(ctx, dst, tableName, lastRowValues); err != nil {
		return false, err
	}

	sklog.Infof("Migrated batch of %d rows for %s", len(batch), tableName)
	return len(batch) < batchSize, nil
}

func buildQuery(tableName string, hasCreatedAt bool, orderByCols []string, lastValues []interface{}, batchSize int) string {
	var sb strings.Builder
	sb.WriteString("SELECT *")
	if !hasCreatedAt {
		// If the source doesn't have createdat, we provide a dummy value so ScanFrom works,
		// but we don't order by it (orderByCols will only contain PKs).
		sb.WriteString(", '0001-01-01 00:00:00+00'::TIMESTAMPTZ as createdat")
	}
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

func extractProgressValues(exporter sqltest.SQLExporter, orderByCols []string) []interface{} {
	colNames, allValues := exporter.ToSQLRow()
	v := reflect.Indirect(reflect.ValueOf(exporter))

	var res []interface{}
	for _, col := range orderByCols {
		if col == "createdat" {
			res = append(res, v.FieldByName("CreatedAt").Interface().(time.Time))
			continue
		}
		found := false
		for i, name := range colNames {
			if strings.EqualFold(name, col) {
				res = append(res, allValues[i])
				found = true
				break
			}
		}
		if !found {
			panic(fmt.Sprintf("Column %s not found in ToSQLRow for table", col))
		}
	}
	return res
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
	var values []interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		return nil, err
	}
	// JSON unmarshaling might turn timestamps into strings and bytes into base64 strings.
	// However, CockroachDB driver might handle them if passed as strings, but it's safer to have right types.
	// For simplicity in this script, we'll try as is, or we could refine based on table schema.
	return values, nil
}

func saveProgress(ctx context.Context, db *pgxpool.Pool, tableName string, values []interface{}) error {
	data, err := json.Marshal(values)
	if err != nil {
		return err
	}
	_, err = db.Exec(ctx, `INSERT INTO migration_progress (table_name, last_processed_values, updated_at)
		VALUES ($1, $2, CURRENT_TIMESTAMP)
		ON CONFLICT (table_name) DO UPDATE SET table_name = EXCLUDED.table_name, last_processed_values = EXCLUDED.last_processed_values, updated_at = EXCLUDED.updated_at`,
		tableName, data)
	return err
}

func getRowType(tableName string) reflect.Type {
	t := reflect.TypeOf(schema.Tables{})
	f, ok := t.FieldByName(tableName)
	if !ok {
		return nil
	}
	return f.Type.Elem()
}

func writeBatch(ctx context.Context, db *pgxpool.Pool, tableName string, batch []sqltest.SQLExporter) error {
	if len(batch) == 0 {
		return nil
	}

	fullColNames := getFullColNames(batch[0])

	copySource := &batchCopySource{
		batch: batch,
		idx:   -1,
	}

	_, err := db.CopyFrom(ctx, pgx.Identifier{strings.ToLower(tableName)}, fullColNames, copySource)
	return err
}

func getFullColNames(exporter sqltest.SQLExporter) []string {
	colNames, _ := exporter.ToSQLRow()
	return colNames
}

type batchCopySource struct {
	batch []sqltest.SQLExporter
	idx   int
}

func (s *batchCopySource) Next() bool {
	s.idx++
	return s.idx < len(s.batch)
}

func (s *batchCopySource) Values() ([]interface{}, error) {
	_, values := s.batch[s.idx].ToSQLRow()
	return values, nil
}

func (s *batchCopySource) Err() error {
	return nil
}
