# Pinpoint SQL Schema and Store

This directory contains the database schema definitions for the Pinpoint service, the schema
generator, the generated SQL, and the data access layer (store).

The database schema is defined using Go structs as the single source of truth.
A generator program then converts these structs into SQL `CREATE TABLE` statements for Spanner.

## Schema Generation

The SQL schema is not written by hand. Instead, it is generated from Go structs to ensure
consistency between the application code and the database.

1.  **Schema Definition**: The schema for each table is defined in a Go struct within
    `schema/schema.go`. Fields are annotated with `sql` tags to specify column types and
    constraints.
2.  **Generator**: The `tosql/main.go` program reads the schema definitions. It uses the
    `go/sql/exporter` utility to convert the Go structs into a Spanner-compatible SQL `CREATE TABLE`
    string.
3.  **Generation Command**: To regenerate the schema, run the generator from within the
    `pinpoint/go/sql` directory:
    `bash
go run ./tosql/main.go
`
4.  **Output**: The command writes the generated SQL schema to `schema/spanner/spanner.go`.

## File Overview

### `schema/schema.go`

This file is the source of truth for the database schema. It defines Go structs, like
`JobSchema`, the struct fields correspond to table columns, and `sql` tags define the
SQL data types.

### `tosql/main.go`

This is an executable that generates the SQL schema. It imports the schema definitions
from `schema/schema.go`, processes them, and writes the resulting SQL DDL to
`schema/spanner/spanner.go`.

### `schema/spanner/spanner.go`

This file is **auto-generated** and should not be edited manually. It contains a Go string
constant named `Schema` with the full `CREATE TABLE` statement for the Pinpoint database.
