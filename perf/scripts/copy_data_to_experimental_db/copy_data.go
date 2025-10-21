package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jackc/pgx/v4"
)

const testInstanceDb = "testdb_test_set_me" // e.g. chromeint_test
const tableName = "tablename_set_me"        // Identifier of the table you want to copy over
// columns you want to copy over
var columnNames = []string{"id", "and", "other", "columns", "set", "me"}

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

func main() {
	ctx := context.Background()

	// 1. Get database connection URLs from environment variables
	sourceDbUrl := "postgresql://root@localhost:5432/chrome_int?sslmode=disable"

	destDbUrl := "postgresql://root@localhost:5433/" + testInstanceDb + "?sslmode=disable"

	// 2. Connect to the source database
	sourceConn, err := pgx.Connect(ctx, sourceDbUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to source database: %v\n", err)
		os.Exit(1)
	}
	defer sourceConn.Close(ctx)
	fmt.Println("Successfully connected to source database.")

	// 3. Connect to the destination database
	destConn, err := pgx.Connect(ctx, destDbUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Unable to connect to destination database: %v\n", err)
		os.Exit(1)
	}
	defer destConn.Close(ctx)
	fmt.Println("Successfully connected to destination database.")

	// For large tables on Cloud Spanner, set autocommit DML mode to partitioned non-atomic.
	// This allows PGAdapter to break large COPY operations into smaller transactions.
	_, err = destConn.Exec(ctx, "SET SPANNER.AUTOCOMMIT_DML_MODE='PARTITIONED_NON_ATOMIC'")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to set Spanner autocommit mode: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("Set SPANNER.AUTOCOMMIT_DML_MODE='PARTITIONED_NON_ATOMIC' on destination database.")

	// 4. Query all rows from the source table
	columns := strings.Join(columnNames, ", ")
	query := fmt.Sprintf("SELECT %s FROM %s", columns, tableName)
	rows, err := sourceConn.Query(ctx, query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Query failed: %v\n", err)
		os.Exit(1)
	}
	// Not deferring rows.Close() because CopyFrom will close it.

	// 5. Use pgx.CopyFrom for efficient bulk insertion
	source := &copySource{rows: rows}
	tableNameId := pgx.Identifier{tableName}

	fmt.Println("Starting data copy...")
	copyCount, err := destConn.CopyFrom(
		ctx,
		tableNameId,
		columnNames,
		source,
	)

	if err != nil {
		fmt.Fprintf(os.Stderr, "CopyFrom failed: %v\n", err)
		// Check for source rows error as well
		if source.Err() != nil {
			fmt.Fprintf(os.Stderr, "Source rows error: %v\n", source.Err())
		}
		os.Exit(1)
	}

	fmt.Printf("Successfully copied %d rows.\n", copyCount)
}
