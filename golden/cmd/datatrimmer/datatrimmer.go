// The datatrimmer executable trims old data that is no longer relevant.
// Example to run:
// `go run ./cmd/datatrimmer --db_name=skiainfra --table_name=tiledtracedigests1 --batch_size=10`
// or `bazelisk run //golden/cmd/datatrimmer -- --db_name...`
package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"text/template"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog/sklogimpl"
	"go.skia.org/infra/go/sklog/stdlogging"

	"go.skia.org/infra/go/sklog"
)

const cockroachDBVersion = "--image=cockroachdb/cockroach:v22.2.3"

var supportedTables = []string{"tiledtracedigests", "valuesathead"}

func main() {
	dryRun := flag.Bool("dry_run", true, "dry run rollback the transaction. Default true.")
	dbCluster := flag.String("db_cluster", "gold-cockroachdb:26234", "name of the cluster")
	dbName := flag.String("db_name", "", "name of database to trim data")
	tableName := flag.String("table_name", "", fmt.Sprintf("name of table to trim data. Supported: %s", supportedTables))
	batchSize := flag.Int("batch_size", 1000, "limit the number of data to be trimmed. Default 1000.")
	cutOffDate := flag.String("cut_off_date", "2024-04-01", "date in YYYY-MM-DD format to determine old data. Default 2024-04-01.")

	sklogimpl.SetLogger(stdlogging.New(os.Stderr))
	flag.Parse()
	if *dbName == "" {
		sklog.Fatalf("Must supply db_name")
	}
	if *dbCluster == "" {
		sklog.Fatalf("Must supply db_cluster")
	}
	if *tableName == "" {
		sklog.Fatalf("Must supply table_name")
	}
	normalizedTableName := strings.ToLower(*tableName)
	if !slices.Contains(supportedTables, normalizedTableName) {
		sklog.Fatalf("Must supply a valid table_name")
	}
	// Both k8s and cockroachdb expect database names to be lowercase.
	normalizedDB := strings.ToLower(*dbName)

	params := &parameters{
		DryRun:     *dryRun,
		DBName:     normalizedDB,
		TableName:  normalizedTableName,
		BatchSize:  *batchSize,
		CutOffDate: *cutOffDate,
	}

	if *dryRun {
		sklog.Infof("Running in DRY RUN mode, use `--dry_run=false` to disable")
	}

	trimmer, _ := getTrimmerForTable(params)
	sql := trimmer.sql()

	sklog.Infof("Trimming %s.%s with SQL:\n%s", normalizedDB, normalizedTableName, sql)

	out, err := exec.Command("kubectl", "run",
		"gold-cockroachdb-datatrimmer-"+normalizedDB,
		"--restart=Never", cockroachDBVersion,
		"--rm", "-i", // -i forces this command to wait until it completes.
		"--", "sql",
		"--insecure", "--host="+*dbCluster,
		"--execute="+sql,
	).CombinedOutput()
	if err != nil {
		sklog.Fatalf("Error while trimming %s.%s: %s %s", normalizedDB, normalizedTableName, err, string(out))
	}

	sklog.Infof("Done with output: %s", string(out))
}

// getTrimmerForTable returns the dataTrimmer for the table specified in params.
func getTrimmerForTable(params *parameters) (dataTrimmer, error) {
	switch params.TableName {
	case "tiledtracedigests":
		return &tiledTraceDigestsTrimmer{params: params}, nil
	case "valuesathead":
		return &valuesAtHeadTrimmer{params: params}, nil
	default:
		return nil, skerr.Fmt("unimplemented for table %s", params.TableName)
	}
}

// generateSQL applies the params values to the sqlTemplate.
func generateSQL(sqlTemplate string, params *parameters) string {
	temp := template.Must(template.New("").Parse(sqlTemplate))
	body := strings.Builder{}
	err := temp.Execute(&body, params)
	if err != nil {
		panic(err)
	}
	sql := body.String()
	return sql
}

// parameters is the data model provided to a dataTrimmer.
type parameters struct {
	DryRun     bool
	DBName     string
	TableName  string
	BatchSize  int
	CutOffDate string
}

// dataTrimmer is the interface that all data trimmers must implement.
// To avoid long transactions and locks, most dataTrimmer implementations should
// - support batch size
// - create a temp table to select the IDs of data to be deleted
// - delete data from the original table in a transaction
type dataTrimmer interface {
	sql() string
}

// tiledTraceDigestsTrimmer implements data trimmer for the TiledTraceDigests table.
type tiledTraceDigestsTrimmer struct {
	params *parameters
}

// sql for the tiledTraceDigestsTrimmer finds old data based on tile_id. It
// creates a temp table for the keys of the data to be trimmed, and then joins
// that table to delete the data efficiently from the original table.
// Note that the actual rows to be deleted are likely to be greater than the
// batch size because we only use tile_id and trace_id, which can be associated
// with multiple digests.
func (t *tiledTraceDigestsTrimmer) sql() string {
	sqlTemplate := `
SET experimental_enable_temp_tables=on;
USE {{.DBName}};
CREATE TEMP TABLE TiledTraceDigests_trimmer (
  tile_id INT4 NOT NULL,
  trace_id BYTES NOT NULL,
  CONSTRAINT "primary" PRIMARY KEY (tile_id ASC, trace_id ASC)
);
WITH
min_commit_id_to_keep AS (
  SELECT commit_id FROM GitCommits
  WHERE commit_time > '{{.CutOffDate}}' ORDER BY commit_time ASC LIMIT 1
),
min_tile_id_to_keep AS (
  SELECT tile_id FROM CommitsWithData cwd
    JOIN min_commit_id_to_keep cid ON cid.commit_id = cwd.commit_id
  LIMIT 1
)
INSERT INTO TiledTraceDigests_trimmer (tile_id, trace_id)
SELECT DISTINCT tile_id, trace_id FROM TiledTraceDigests
WHERE tile_id < (SELECT tile_id FROM min_tile_id_to_keep)
ORDER BY tile_id ASC LIMIT {{.BatchSize}};

BEGIN;
DELETE FROM TiledTraceDigests o
WHERE EXISTS (
  SELECT 1 FROM TiledTraceDigests_trimmer t
  WHERE o.tile_id = t.tile_id AND o.trace_id = t.trace_id
)
RETURNING o.tile_id, o.trace_id;
`
	if t.params.DryRun {
		sqlTemplate += `
ROLLBACK;
`
	} else {
		sqlTemplate += `
COMMIT;
`
	}
	sql := generateSQL(sqlTemplate, t.params)
	return sql
}

// valuesAtHeadTrimmer implements data trimmer for the ValuesAtHead table.
type valuesAtHeadTrimmer struct {
	params *parameters
}

// sql for the valuesAtHeadTrimmer finds old data based on commit_id.
// Note that the corpus condition in the where clause is a performance tweak.
// See http://go/scrcast/NjY4MjA5MDQwMDY0NTEyMHxkNzYzOWEyMi02Mw for more info.
func (t *valuesAtHeadTrimmer) sql() string {
	sqlTemplate := `
SET experimental_enable_temp_tables=on;
USE {{.DBName}};
CREATE TEMP TABLE ValuesAtHead_trimmer (
  trace_id BYTES NOT NULL,
  CONSTRAINT "primary" PRIMARY KEY (trace_id ASC)
);
WITH
min_commit_id_to_keep AS (
  SELECT commit_id FROM GitCommits
  WHERE commit_time > '{{.CutOffDate}}' ORDER BY commit_time ASC LIMIT 1
)
INSERT INTO ValuesAtHead_trimmer (trace_id)
SELECT trace_id FROM ValuesAtHead
WHERE corpus IN (SELECT DISTINCT keys->>'source_type' AS corpus FROM Groupings)
  AND most_recent_commit_id < (SELECT commit_id FROM min_commit_id_to_keep)
LIMIT {{.BatchSize}};

BEGIN;
DELETE FROM ValuesAtHead o
WHERE EXISTS (
  SELECT 1 FROM ValuesAtHead_trimmer t
  WHERE o.trace_id = t.trace_id
)
RETURNING o.trace_id;
`
	if t.params.DryRun {
		sqlTemplate += `
ROLLBACK;
`
	} else {
		sqlTemplate += `
COMMIT;
`
	}
	sql := generateSQL(sqlTemplate, t.params)
	return sql
}
