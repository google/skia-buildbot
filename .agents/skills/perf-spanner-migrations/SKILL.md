---
name: perf spanner migrations
description: >-
  Instructions for performing database schema migrations, modifying tables,
  and regenerating Spanner schema files in the Performance Dashboard (perf).
---

# Performance Dashboard (perf) Cloud Spanner Schema Migrations

Use this skill when modifying the database schema, adding new database tables, writing SQL migrations, or regenerating Go/JSON schema target files in `perf/go/sql`.

## Workflow for Schema Changes

When modifying the database schema, follow these sequential steps:

### 1. Create a New Migration File

Add a new sequence-numbered SQL file inside `perf/go/sql/expectedschema/migrations/`:

- Format: `000X_description.sql` (e.g., `0004_add_metadata_column.sql`).
- Sequence numbers must be strictly sequential (no gaps or duplicate versions).
- Cloud Spanner has strict schema modification constraints (e.g., you cannot change a column's type to `ARRAY` or to incompatible types directly; add a new column instead).

### 2. Update Table Structs

Modify the Go structs representing the table layouts in [tables.go](../../../perf/go/sql/tables.go) to match the new schema layout.

### 3. Regenerate the Go Schema File

Regenerate [schema_spanner.go](../../../perf/go/sql/spanner/schema_spanner.go) from the structs:

```bash
cd perf/go/sql
go generate
```

_(This executes `//perf/go/sql/tosql` under the hood to write the target SQL declarations)._

### 4. Regenerate the expected JSON Catalog Description

Regenerate [schema_spanner.json](../../../perf/go/sql/expectedschema/schema_spanner.json):

> [!IMPORTANT]
> Because Bazel runs commands inside sandboxed output directories, passing a relative path to `--out` will write the file into Bazel's output cache instead of your actual workspace.
> **Always use an absolute path** (e.g. `$(pwd)/...` or `$(git rev-parse --show-toplevel)/...`) for the output file:

```bash
cd perf
bazelisk run --config=mayberemote //perf/go/sql/exportschema -- \
    --out $(pwd)/go/sql/expectedschema/schema_spanner.json \
    --databaseType spanner
```

### 5. Verify and Run Unit Tests

To run database tests locally against the emulator:

1. **Launch a clean Spanner Emulator instance**:
   ```bash
   make -C perf run-spanner-emulator
   ```
2. **Export the connection environment variables**:
   ```bash
   export PGADAPTER_HOST=localhost:5432
   export SPANNER_EMULATOR_HOST=localhost:9010
   ```
3. **Run the SQL package tests**:
   ```bash
   bazelisk test //perf/go/sql/...
   ```
