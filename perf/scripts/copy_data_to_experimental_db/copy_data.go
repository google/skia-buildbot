package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/perf/go/sql/spanner"
)

// tableToColumns maps table names to their respective column names.
var tableToColumns = map[string][]string{
	// go/keep-sorted start
	"anomalygroups":   spanner.AnomalyGroups,
	"commits":         spanner.Commits,
	"culprits":        spanner.Culprits,
	"favorites":       spanner.Favorites,
	"graphsshortcuts": spanner.GraphsShortcuts,
	"metadata":        spanner.Metadata,
	"paramsets":       spanner.ParamSets,
	"postings":        spanner.Postings,
	"regressions":     spanner.Regressions,
	"regressions2":    spanner.Regressions2,
	"reversekeymap":   spanner.ReverseKeyMap,
	"shortcuts":       spanner.Shortcuts,
	"sourcefiles":     spanner.SourceFiles,
	"subscriptions":   spanner.Subscriptions,
	"traceparams":     spanner.TraceParams,
	"tracevalues":     spanner.TraceValues,
	"userissues":      spanner.UserIssues,
	// go/keep-sorted end
}

// copySource is a helper struct that implements the pgx.CopyFromSource interface.
type copySource struct {
	rows pgx.Rows
}

// Next implements the pgx.CopyFromSource interface.
func (c *copySource) Next() bool {
	return c.rows.Next()
}

// Values implements the pgx.CopyFromSource interface.
func (c *copySource) Values() ([]interface{}, error) {
	return c.rows.Values()
}

// Err implements the pgx.CopyFromSource interface.
func (c *copySource) Err() error {
	return c.rows.Err()
}

// parseFlags parses and validates the command-line flags.
func parseFlags() (string, string, string) {
	durationStr := flag.String("duration", "", "Duration to copy data for, e.g., '168h' for last week or 'all' to copy all data.")
	dbName := flag.String("db-name", "srudenkov", "The name of the test instance database.")
	tableName := flag.String("table-name", "regressions2", "Identifier of the table you want to copy over, or 'all' to copy all tables.")
	flag.Parse()

	if *durationStr == "" {
		fmt.Fprintln(os.Stderr, "Error: --duration flag is required.")
		flag.Usage()
		os.Exit(1)
	}
	return *durationStr, *dbName, *tableName
}

// connectToDB connects to a PostgreSQL database and returns the connection object.
func connectToDB(ctx context.Context, dbURL string) (*pgx.Conn, error) {
	conn, err := pgx.Connect(ctx, dbURL)
	if err != nil {
		return nil, fmt.Errorf("unable to connect to database: %w", err)
	}
	fmt.Printf("Successfully connected to %s.\n", dbURL)
	return conn, nil
}

// checkExistingData checks if the destination table already contains data for the specified duration.
func checkExistingData(ctx context.Context, destConn *pgx.Conn, tableName, durationStr string) error {
	if tableName == "tracevalues" {
		fmt.Println("No data duplication check is performed for tracevalues. Assume you know what you are doing.")
		return nil
	}
	var count int
	var countQuery string
	var countArgs []interface{}

	countQuery = fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)

	if durationStr != "all" {
		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			return fmt.Errorf("invalid duration format: %w", err)
		}
		since := time.Now().Add(-duration)
		countQuery += " WHERE createdat > $1"
		countArgs = append(countArgs, since)
	}

	err := destConn.QueryRow(ctx, countQuery, countArgs...).Scan(&count)
	if err != nil {
		if strings.Contains(err.Error(), "does not exist") {
			return nil
		}
		return fmt.Errorf("failed to check for existing data: %w", err)
	}
	if count > 0 {
		return fmt.Errorf("destination table %s already contains data for the specified duration", tableName)
	}
	return nil
}

// copyData performs the data copy operation from the source to the destination table.
func copyData(ctx context.Context, sourceConn, destConn *pgx.Conn, tableName, durationStr string) error {
	columnNames, ok := tableToColumns[tableName]
	if !ok {
		return fmt.Errorf("unknown table: %s", tableName)
	}

	var query string
	var args []interface{}

	// The tracevalues table is very large (8TB+ in chrome_int).
	// To optimize, we filter them based on the creation time of their associated source files
	// rather than their own creation time.
	if tableName == "tracevalues" {
		columns := "t." + strings.Join(columnNames, ", t.")
		query = fmt.Sprintf(`
			SELECT %s
			FROM %s t
			JOIN sourcefiles s ON t.source_file_id = s.source_file_id`, columns, tableName)
	} else {
		columns := strings.Join(columnNames, ", ")
		query = fmt.Sprintf("SELECT %s FROM %s", columns, tableName)
	}

	if durationStr != "all" {
		duration, err := time.ParseDuration(durationStr)
		if err != nil {
			return fmt.Errorf("invalid duration format: %w", err)
		}
		since := time.Now().Add(-duration)
		if tableName == "tracevalues" {
			query += " WHERE s.createdat > $1"
		} else {
			query += " WHERE createdat > $1"
		}
		args = append(args, since)
	}

	rows, err := sourceConn.Query(ctx, query, args...)
	if err != nil {
		return fmt.Errorf("query failed: %w", err)
	}

	source := &copySource{rows: rows}
	tableNameId := pgx.Identifier{tableName}

	fmt.Printf("Starting data copy for table %s...\n", tableName)
	copyCount, err := destConn.CopyFrom(
		ctx,
		tableNameId,
		columnNames,
		source,
	)
	if err != nil {
		if source.Err() != nil {
			return fmt.Errorf("CopyFrom failed: %v, Source rows error: %v", err, source.Err())
		}
		return fmt.Errorf("CopyFrom failed: %w", err)
	}

	fmt.Printf("Successfully copied %d rows to table %s.\n", copyCount, tableName)
	return nil
}

// processTable orchestrates the data copying process for a single table.
func processTable(ctx context.Context, sourceConn, destConn *pgx.Conn, tableName, durationStr string) error {
	if err := checkExistingData(ctx, destConn, tableName, durationStr); err != nil {
		return err
	}
	return copyData(ctx, sourceConn, destConn, tableName, durationStr)
}

func main() {
	durationStr, dbName, tableName := parseFlags()
	ctx := context.Background()

	sourceDbURL := "postgresql://root@localhost:5432/chrome_int?sslmode=disable"
	destDbURL := "postgresql://root@localhost:5433/" + dbName + "?sslmode=disable"

	sourceConn, err := connectToDB(ctx, sourceDbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer sourceConn.Close(ctx)

	destConn, err := connectToDB(ctx, destDbURL)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%v\n", err)
		os.Exit(1)
	}
	defer destConn.Close(ctx)

	_, err = destConn.Exec(ctx, "SET SPANNER.AUTOCOMMIT_DML_MODE='PARTITIONED_NON_ATOMIC'")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set Spanner autocommit mode: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Set SPANNER.AUTOCOMMIT_DML_MODE='PARTITIONED_NON_ATOMIC' on destination database.")

	if tableName == "all" {
		for table := range tableToColumns {
			if err := processTable(ctx, sourceConn, destConn, table, durationStr); err != nil {
				fmt.Fprintf(os.Stderr, "Error copying table %s: %v\n", table, err)
			}
		}
	} else {
		if err := processTable(ctx, sourceConn, destConn, tableName, durationStr); err != nil {
			fmt.Fprintf(os.Stderr, "Error copying table %s: %v\n", tableName, err)
			os.Exit(1)
		}
	}
}
