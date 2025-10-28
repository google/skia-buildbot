package exporter

import (
	"fmt"
	"reflect"
	"slices"
	"strings"
)

// Options to control the generation done by GenerateSQL.
type Options int

const (
	SchemaOnly Options = iota
	SchemaAndColumnNames
)

// SchemaTarget defines the target db to generate the schema for.
type SchemaTarget int

const (
	CockroachDB SchemaTarget = iota
	Spanner
)

// SpannerConverter provides a struct to help replace necessary values
// in the schema to be compatible with Spanner postgres.
// TODO(ashwinpv): This will replace the other schema gen once spanner is fully rolled out.
type SpannerConverter struct {
	sequences        []string
	indices          []string
	indexNames       []string
	TtlExcludeTables []string
	primaryKeys      map[string]string
	SkipCreatedAt    bool // If true, doesn't add a createdat column to the table.
	GoogleSQL        bool
	interleaves      map[string]string
}

func DefaultSpannerConverter() *SpannerConverter {
	return &SpannerConverter{
		sequences:        []string{},
		indices:          []string{},
		indexNames:       []string{},
		TtlExcludeTables: []string{},
		primaryKeys:      map[string]string{},
		interleaves:      map[string]string{},
	}
}

// getSequenceDeclarations returns the sequence creation statements for all
// the sequences that were encountered while replacing unique_rowid() during schema generation.
func (sc *SpannerConverter) getSequenceDeclarations() string {
	if len(sc.sequences) > 0 {
		sequenceBuilder := strings.Builder{}
		for _, seq := range sc.sequences {
			sequenceBuilder.WriteString("CREATE SEQUENCE IF NOT EXISTS " + seq + " bit_reversed_positive;\n")
		}

		return sequenceBuilder.String()
	}

	return ""
}

// getIndexDeclarations returns the index creation statements for all
// the indices that were encountered during schema generation.
func (sc *SpannerConverter) getIndexDeclarations() string {
	if len(sc.indices) > 0 {
		indexBuilder := strings.Builder{}
		for _, idx := range sc.indices {
			if sc.GoogleSQL {
				indexBuilder.WriteString("CREATE UNIQUE INDEX IF NOT EXISTS " + idx + ";\n")
			} else {
				indexBuilder.WriteString("CREATE INDEX IF NOT EXISTS " + idx + ";\n")
			}
		}

		return indexBuilder.String()
	}

	return ""
}

// updateColumnTypesForSpanner updates the column types defined in the column string with types
// compatible with Spanner postgres.
func (sc *SpannerConverter) updateColumnTypesForSpanner(sqlColumnText string, tableName string) string {
	var typeReplacements map[string]string
	if sc.GoogleSQL {
		if strings.Contains(sqlColumnText, "PRIMARY KEY (") {
			pkColumnStr := strings.Split(sqlColumnText, "PRIMARY KEY (")[1]
			pkColumnStr = strings.Split(pkColumnStr, ")")[0]
			sc.primaryKeys[tableName] = pkColumnStr
			return ""
		}
		if strings.Contains(sqlColumnText, "INTERLEAVE") {
			sc.interleaves[tableName] = sqlColumnText
			return ""
		}
	}
	if !sc.GoogleSQL {
		typeReplacements = map[string]string{
			"INT2":              "INT8",
			"INT4":              "INT8",
			"CHAR":              "VARCHAR(1)",
			"STRING":            "TEXT",
			"UUID":              "TEXT",
			"BYTES":             "BYTEA",
			"gen_random_uuid()": "spanner.generate_uuid()",
			"UNIQUE":            "",
			"SERIAL":            "INT8",
		}

		// unique_rowid() generates a unique integer identifier for int columns. This does not work in spanner.
		// The replacement is basically to define a SEQUENCE and use nextval('<sequence_name>') to get the
		// unique value. We keep a track of all the sequences we need to create and then create the generation
		// statements later.
		uniqueRowIdentifier := "unique_rowid()"
		if strings.Contains(sqlColumnText, uniqueRowIdentifier) {
			sequenceName := fmt.Sprintf("%s_seq", tableName)
			sc.sequences = append(sc.sequences, sequenceName)

			typeReplacements[uniqueRowIdentifier] = fmt.Sprintf("nextval('%s')", sequenceName)
			if strings.Contains(sqlColumnText, "PRIMARY KEY") {
				// The primary key statement should come after the columns when we
				// are using nextval for generating unique row ids.
				columnName := strings.Split(sqlColumnText, " ")[0]
				sc.primaryKeys[tableName] = columnName
				sqlColumnText = strings.Replace(sqlColumnText, " PRIMARY KEY", "", -1)
			}
		}
	}
	if strings.Contains(sqlColumnText, "INDEX") {

		// This is a list of indices to ignore. When we switch to spanner,
		// these will either be removed or updated with a compatible replacement.
		ignoreIndices := map[string][]string{
			// These are not supported since spanner does not support indexing on JSONB objects.
			"Traces":       {"keys_idx", "keys_idx_1"},
			"ValuesAtHead": {"keys_idx"},
		}
		// Index is specified as "INDEX <index_name> (index columns)"
		indexSpecStartIdx := strings.Index(sqlColumnText, "INDEX") + 6
		indexSpec := sqlColumnText[indexSpecStartIdx:]
		splits := strings.SplitAfterN(indexSpec, " ", 2)
		indexName := strings.TrimSpace(splits[0])
		indexDetails := strings.TrimSpace(splits[1])

		if strings.Contains(indexDetails, "STORING") {
			indexDetails = strings.ReplaceAll(indexDetails, "STORING", "INCLUDE")
		}

		if slices.Contains(sc.indexNames, indexName) {
			indexName = indexName + "_1"
		}

		if excludeIndices, ok := ignoreIndices[tableName]; ok {
			if slices.Contains(excludeIndices, indexName) {
				return ""
			}
		}
		sc.indexNames = append(sc.indexNames, indexName)

		// Spanner expects the index definition to be "CREATE INDEX <index_name> on <table_name> (<columns>)"
		sc.indices = append(sc.indices, fmt.Sprintf("%s on %s %s", indexName, tableName, indexDetails))

		// The index is not specified in the schema as a column, so return empty string.
		return ""
	}

	// Check if this is a computed column in CDB schema.
	// Eg: corpus TEXT AS (keys->>'source_type') STORED NOT NULL
	// This should be written as "corpus TEXT GENERATED ALWAYS AS (keys->>'source_type') STORED NOT NULL" for Spanner.
	if strings.Contains(sqlColumnText, "AS (") {
		insertIndex := strings.Index(sqlColumnText, "AS (")
		sqlColumnText = sqlColumnText[:insertIndex-1] + " GENERATED ALWAYS " + sqlColumnText[insertIndex:]
	}

	updatedString := sqlColumnText
	for baseType, spannerType := range typeReplacements {
		if strings.Contains(sqlColumnText, baseType) {
			updatedString = strings.Replace(updatedString, baseType, spannerType, 1)
		}
	}

	return updatedString
}

// GenerateSQL takes in a "table type", that is a table whose fields are slices.
// Each field will be interpreted as a table. The sql struct tags will be used
// to generate the SQL schema. A package name is taken in to be included in the
// returned string. If a malformed type is passed in, this function will panic.
func GenerateSQL(inputType interface{}, pkg string, opt Options, schemaTarget SchemaTarget, spannerConverter *SpannerConverter) string {
	header := fmt.Sprintf("package %s\n\n// Generated by //go/sql/exporter/\n// DO NOT EDIT\n\nconst Schema = `", pkg)
	var sc *SpannerConverter
	if schemaTarget == Spanner {
		if spannerConverter != nil {
			sc = spannerConverter
		} else {
			sc = DefaultSpannerConverter()
		}
	}
	body := strings.Builder{}
	t := reflect.TypeOf(inputType)
	for i := 0; i < t.NumField(); i++ {
		table := t.Field(i) // Fields of the outer type are expected to be tables.
		if table.Type.Kind() != reflect.Slice {
			panic(`Expected table should be a slice: ` + table.Name)
		}
		body.WriteString("CREATE TABLE IF NOT EXISTS ")
		body.WriteString(table.Name)
		body.WriteString(" (")
		row := table.Type.Elem()
		wasFirst := true
		for j := 0; j < row.NumField(); j++ {
			col := row.Field(j)
			sqlText, ok := col.Tag.Lookup("sql")
			if !ok {
				panic(`Field missing "sql" tag:` + table.Name + "." + row.Name())
			}
			// If generating for spanner, update the column types to be compatible.
			if sc != nil {
				sqlText = sc.updateColumnTypesForSpanner(sqlText, table.Name)
			}
			// If the column was index specification and we are generating for spanner,
			// sqlText can be empty.
			if sqlText != "" {
				if !wasFirst {
					body.WriteString(",")
				}
				wasFirst = false
				body.WriteString("\n  ")

				body.WriteString(strings.TrimSpace(sqlText))
			}
		}
		if sc != nil {
			skipClose := false
			if !sc.SkipCreatedAt {
				// Automatically create a TTL column for tables in Spanner.
				body.WriteString(",\n  createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP")
			}
			if pk, ok := sc.primaryKeys[table.Name]; ok {
				if sc.GoogleSQL {
					body.WriteString("\n) PRIMARY KEY (" + pk + ")")
					if sc.interleaves[table.Name] != "" {
						body.WriteString(",\n  ")
						body.WriteString(sc.interleaves[table.Name])
					}

					body.WriteString(";\n")
					skipClose = true
				} else {
					body.WriteString(",\n  PRIMARY KEY (" + pk + ")")
				}
			}
			// Do not add a TTL spec if the table is excluded.
			if slices.Contains(sc.TtlExcludeTables, table.Name) {
				if !skipClose {
					body.WriteString("\n);\n")
				}
			} else {
				// Add TTL spec of 3 years by default.
				body.WriteString("\n) TTL INTERVAL '1095 days' ON createdat;\n")
			}
		} else {
			body.WriteString("\n);\n")
		}
	}

	sequences := ""
	indices := ""
	// If generating for spanner, we would need to add sequence creation statements before
	// the tables for all unique_rowid() replacements.
	if schemaTarget == Spanner {
		sequences = sc.getSequenceDeclarations()
		indices = sc.getIndexDeclarations()

		body.WriteString(indices)
	}

	body.WriteString("`\n")
	cols := ""
	if opt == SchemaAndColumnNames {
		cols += columnNames(inputType)
	}

	return header + sequences + body.String() + cols
}

// columnNames takes in a "table type", that is a table whose fields are slices.
// Each field will be interpreted as a table. The sql struct tags will be used
// to generate a variable for each table that contains the column names in the
// order they appear in the struct. If a malformed type is passed in, this
// function will panic.
//
// Indexes and computed columns are ignored.
func columnNames(inputType interface{}) string {
	body := strings.Builder{}

	t := reflect.TypeOf(inputType)
	for i := 0; i < t.NumField(); i++ {
		body.WriteString("\n")
		table := t.Field(i) // Fields of the outer type are expected to be tables.
		if table.Type.Kind() != reflect.Slice {
			panic(`Expected table should be a slice: ` + table.Name)
		}
		body.WriteString(`var `)
		body.WriteString(table.Name)
		body.WriteString(" = []string{")
		row := table.Type.Elem()
		for j := 0; j < row.NumField(); j++ {
			col := row.Field(j)
			sqlText, ok := col.Tag.Lookup("sql")
			if !ok {
				panic(`Field missing "sql" tag:` + table.Name + "." + row.Name())
			}
			sqlText = strings.TrimSpace(sqlText)
			if strings.Contains(sqlText, "STORED") || strings.HasPrefix(sqlText, "INDEX") || strings.HasPrefix(sqlText, "PRIMARY") || strings.HasPrefix(sqlText, "INVERTED") {
				continue
			}
			body.WriteString("\n")
			colName := strings.SplitN(sqlText, " ", 2)[0]
			body.WriteString("\t\"")
			body.WriteString(colName)
			body.WriteString(`",`)
		}
		body.WriteString("\n}\n")
	}
	return body.String()
}
