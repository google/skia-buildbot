# Copy Spanner Data

This suite of scripts allows you to copy data from a production database to an experimental one.

## Prerequisites

- You have created a database in an experimental Spanner instance.
- You have set up the necessary tables in your experimental database. You can do this in one of two ways:
  - **For a single table:** Find the DDL by navigating to the source table schema in the Cloud Console and clicking "Show equivalent DDL". [example](https://pantheon.corp.google.com/spanner/instances/tfgen-spanid-20241205020733610/databases/chrome_int/tables/regressions2/details/schema?chat=true&e=-13802955&mods=component_inspector&project=skia-infra-corp)
  - **For all tables:** Execute the DDL statements found in `perf/go/sql/spanner/schema_spanner.go` in your experimental Spanner database. This will create all tables required by the `copy_data` script when using the `--table-name all` option.

## Instructions

### 1. Run PGAdapter Containers

The `run_two_spanners.sh` script starts two PGAdapter Docker containers, one for the source and one for the destination Spanner instance.

**Usage:**

```bash
./run_two_spanners.sh -di <instance2> -dd <database2> [-si <instance1>] [-sd <database1>]
```

**Arguments:**

- `-si`, `--source-instance`: The ID of the source Spanner instance. (Default: `tfgen-spanid-20241205020733610`)
- `-sd`, `--source-database`: The name of the source database. (Default: `chrome_int`)
- `-di`, `--destination-instance`: (Required) The ID of the destination Spanner instance.
- `-dd`, `--destination-database`: (Required) The name of the destination database.

This will expose the source database on `localhost:5432` and the destination database on `localhost:5433`.

### 2. Copy the Data

The `copy_data.go` script copies data from the source table to the destination table.

**Usage:**

First, build the script:

```bash
go build copy_data.go
```

Then, run the executable with the desired flags:

```bash
./copy_data --table-name <table_name> --db-name <db_name> --duration <duration>
```

**Arguments:**

- `--table-name`: (Required) The name of the table to copy (e.g., `regressions2`), or `all` to copy all tables. The table name must be lowercase.
- `--db-name`: (Required) The name of the destination database. This should match the `-d2` value you used with `run_two_spanners.sh`.
- `--duration`: (Required) A duration to specify how far back in time to copy data (e.g., `168h` for the last 7 days) or `all` to copy all data.

The script automatically fetches the correct column names based on the provided table name.

## Example Workflow

1.  **Start the PGAdapters:**
    Start the PGAdapter containers, connecting to the default production instance and your experimental instance `my-instance` with database `my-test-db`.

    ```bash
    ./run_two_spanners.sh -i2 my-instance -d2 my-test-db
    ```

2.  **Create the destination table:**
    In the Spanner console for your `my-test-db` database, create the `regressions2` table using the DDL from the production `chrome_int` database.

3.  **Copy the data for a single table:**
    Build and run the copy script to copy the last week of data from the `regressions2` table.

    ```bash
    go build copy_data.go
    ./copy_data --table-name regressions2 --db-name my-test-db --duration 168h
    ```

4.  **Copy the data for all tables:**
    Build and run the copy script to copy the last week of data from all tables.

    ```bash
    go build copy_data.go
    ./copy_data --table-name all --db-name my-test-db --duration 168h
    ```

## Note

While some checks are in place, this script should not be blindly trusted. It is strongly recommended that the service account used does not have create or delete permissions on the source database.
