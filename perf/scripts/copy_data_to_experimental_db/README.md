This suite of scripts allows you to copy data from production db to an experimental one.

Assumptions:

- You have created a database in the [experimental](https://pantheon.corp.google.com/spanner/instances/tfgen-spanid-20250415224933743/details/databases?chat=true&e=-13802955&mods=component_inspector&project=skia-infra-corp) instance.
- You have setup an empty table with the same name as the production one
- You can find the DDL by nagivating to the source table schema [example](https://pantheon.corp.google.com/spanner/instances/tfgen-spanid-20241205020733610/databases/chrome_int/tables/regressions2/details/schema?chat=true&e=-13802955&mods=component_inspector&project=skia-infra-corp) and pressing "Show equivalent DDL".

First, you should execute `./run_two_spanners.sh` - this will start pgadapters to the source and destination dbs.
You need to modify the script to connect to the right instances.

Next, you should change the global constants/variables in `copy_data.go`.

- `testInstanceDb` should hold the name of destination db you are connecting to.
- `tableName` holds the name of the table you are copying.
- `columnNames` lists the columns that will be copied (most likely, all columns of the table).

After you have changed those variables, you can generate an executable by cd-ing to this folder and e.g. running `go build`.
Simply run the script and wait a few moments while CopyFrom does all the work.

## Note

This script does not perform any validations whatsoever. It is strongly recommended that you don't have create (delete) permissions on the source db.

## Example

- Replace `TEST_DB_HERE` with `mordeckimarcin_test` inside `run_two_spanners.sh` and execute the script.
- Create `regressions2` table inside `mordeckimarcin_test` with the following DDL: (you can execute it in Spanner studio, in experimental instance)

```lang=sql
CREATE TABLE regressions2 (
  id character varying DEFAULT spanner.generate_uuid() NOT NULL,
  commit_number bigint,
  prev_commit_number bigint,
  alert_id bigint,
  creation_time timestamp with time zone,
  median_before real,
  median_after real,
  is_improvement boolean,
  cluster_type character varying,
  cluster_summary jsonb,
  frame jsonb,
  triage_status character varying,
  triage_message character varying,
  createdat timestamp with time zone,
  PRIMARY KEY(id)
);
```

- set `testInstanceDb` with `mordeckimarcin_test`
- set `tableName` with `regressions2`
- set `columnNames` with `[]string{"id", "commit_number", "prev_commit_number", "alert_id", "creation_time", "median_before", "median_after", "is_improvement", "cluster_type", "cluster_summary", "frame", "triage_status", "triage_message", "createdat"}`
- build the script (e.g. `go build`) and execute.
