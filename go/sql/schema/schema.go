// Package schema enables checking the schema, both columns and indexes, of a database.
package schema

import (
	"context"
	"reflect"
	"strings"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sql/pool"
)

const (
	// Timeout used on Contexts when making SQL requests.
	sqlTimeout = time.Minute
)

// TableNames takes in a "table type", that is a table whose fields are slices.
// Each field will be interpreted as a table. TableNames returns all expected
// table names in a string slice with table names lowercased.
//
// For example:
//
//	"description", "taskresult"
func TableNames(tables interface{}) []string {
	ret := []string{}
	for _, structField := range reflect.VisibleFields(reflect.TypeOf(tables)) {
		ret = append(ret, strings.ToLower(structField.Name))
	}
	return ret
}

// Description describes the schema for all tables.
type Description struct {
	ColumnNameAndType map[string]string
	IndexNames        []string
}

// Query to return the typesQuery for each column in all tables.
const typesQuery = `
SELECT
    column_name,
    CONCAT(data_type, ' def:', column_default, ' nullable:', is_nullable)
FROM
    information_schema.columns
WHERE
    table_name = $1;
`

// Query to return the index names for each table.
const indexNameQuery = `
SELECT DISTINCT
	index_name
FROM
	information_schema.statistics
WHERE
	table_name = $1
ORDER BY
	index_name DESC
`

// GetDescription returns a Description populated for every table listed in
// `tables`.
func GetDescription(ctx context.Context, db pool.Pool, tables interface{}) (*Description, error) {
	ctx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()
	colNameAndType := map[string]string{}
	indexNames := []string{}
	for _, tableName := range TableNames(tables) {
		// Fill in colNameAndType.
		rows, err := db.Query(ctx, typesQuery, tableName)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		for rows.Next() {
			var colName string
			var colType string
			err := rows.Scan(&colName, &colType)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			colNameAndType[tableName+"."+colName] = colType
		}

		// Fill in indexNames.
		rows, err = db.Query(ctx, indexNameQuery, tableName)
		if err != nil {
			return nil, skerr.Wrap(err)
		}
		for rows.Next() {
			var indexName string
			err := rows.Scan(&indexName)
			if err != nil {
				return nil, skerr.Wrap(err)
			}
			// In version 22.1 CDB changed the name of the primary key from
			// "primary" to <table_name>_pkey:
			// https://github.com/cockroachdb/cockroach/pull/70604.
			//
			// But if you created a table in a version before 22.1 then the name
			// of the key is preserved as "primary" going forward. Since we know
			// every table will have a primary key the actual name isn't giving
			// us any useful information so we just ignore it regardless of the
			// name.
			if indexName == "primary" || indexName == tableName+"_pkey" {
				continue
			}
			indexNames = append(indexNames, tableName+"."+indexName)
		}
	}

	return &Description{
		ColumnNameAndType: colNameAndType,
		IndexNames:        indexNames,
	}, nil
}
