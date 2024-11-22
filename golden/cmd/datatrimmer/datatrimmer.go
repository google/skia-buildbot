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

var supportedTables = []string{"tiledtracedigests", "valuesathead", "expectations", "tracevalues"}

func main() {
	dryRun := flag.Bool("dry_run", true, "dry run rollback the transaction. Default true.")
	dbCluster := flag.String("db_cluster", "gold-cockroachdb:26234", "name of the cluster")
	dbName := flag.String("db_name", "", "name of database to trim data")
	tableName := flag.String("table_name", "", fmt.Sprintf("name of table to trim data. Supported: %s", supportedTables))
	batchSize := flag.Int("batch_size", 1000, "limit the number of data to be trimmed. Default 1000.")
	cutOffDate := flag.String("cut_off_date", "2024-04-01", "date in YYYY-MM-DD format to determine old data. Default 2024-04-01.")
	corpus := flag.String("corpus", "", "corpus name. Required by the following trimmers: expectations.")

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
		Corpus:     *corpus,
	}

	trimmer, _ := getTrimmerForTable(params)
	sql := trimmer.sql()

	if *dryRun {
		sklog.Infof("Running in DRY RUN mode, use `--dry_run=false` to disable")
	}

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
	case "expectations":
		return &expectationsTrimmer{params: params}, nil
	case "tracevalues":
		return &traceValuesTrimmer{params: params}, nil
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
	Corpus     string
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
);
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
);
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

// expectationsTrimmer implements data trimmer for the Expectations table.
type expectationsTrimmer struct {
	params *parameters
}

// sql for the expectationsTrimmer finds "fresh" digests from the ValuesAtHead
// table and non-fresh untriaged digests are considered safe to delete.
// This trimmer handles one corpus at a time to achieve ideal performance.
func (t *expectationsTrimmer) sql() string {
	if t.params.Corpus == "" {
		sklog.Fatalf("Must supply corpus")
	}

	sqlTemplate := `
SET experimental_enable_temp_tables=on;
USE {{.DBName}};
CREATE TEMP TABLE Expectations_trimmer (
  grouping_id BYTES NOT NULL,
  digest BYTES NOT NULL,
  CONSTRAINT "primary" PRIMARY KEY (grouping_id ASC, digest ASC)
);
WITH
corpus_grouping AS (
  SELECT grouping_id FROM Groupings WHERE keys->>'source_type'='{{.Corpus}}'
),
min_commit_id_to_keep AS (
  SELECT commit_id FROM GitCommits
  WHERE commit_time > '{{.CutOffDate}}' ORDER BY commit_time ASC LIMIT 1
),
fresh_digests AS (
  SELECT DISTINCT grouping_id, digest FROM ValuesAtHead
  WHERE corpus = '{{.Corpus}}'
    AND most_recent_commit_id > (SELECT commit_id FROM min_commit_id_to_keep)
)
INSERT INTO Expectations_trimmer (grouping_id, digest)
SELECT grouping_id, digest FROM Expectations e
WHERE grouping_id IN (SELECT grouping_id FROM corpus_grouping)
  AND e.label = 'u' AND e.expectation_record_id IS NULL
  AND NOT EXISTS (SELECT 1 FROM fresh_digests f WHERE e.digest = f.digest)
LIMIT {{.BatchSize}};

BEGIN;
DELETE FROM Expectations o
WHERE EXISTS (
  SELECT 1 FROM Expectations_trimmer t
  WHERE o.grouping_id = t.grouping_id AND o.digest = t.digest
);
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

// traceValuesTrimmer implements data trimmer for the TraceValues table.
type traceValuesTrimmer struct {
	params *parameters
}

// sql for the traceValuesTrimmer determines old data by commit_id.
// To achieve better performance:
//   - The trimmer only looks back one year from the cut-off date. To trim data
//     more than one year, change the cut-off date for each year.
//   - The `WHERE shard IN` condition is a performance tweak. See
//     http://gpaste/5285242229489664 for more information.
//   - The trimming is based on shard and commit_id, instead of shard, commit_id
//     and trace_id. The former is much faster, but the number of rows to be
//     deleted will be significantly greater than the batch size: a batch of 10
//     commits may trim rows in millions.
func (t *traceValuesTrimmer) sql() string {
	if t.params.BatchSize > 100 {
		// Testing shows a batch size of 10 finishes in ~2 mins, 100 in ~15 mins
		sklog.Warningf("Batch size might be too big for the current trimmer")
	}
	sqlTemplate := `
SET experimental_enable_temp_tables=on;
USE {{.DBName}};
CREATE TEMP TABLE TraceValues_trimmer (
  shard INT2 NOT NULL,
  commit_id STRING NOT NULL,
  CONSTRAINT "primary" PRIMARY KEY (shard ASC, commit_id ASC)
);
WITH
last_year_commits AS (
  SELECT commit_id FROM GitCommits
  WHERE commit_time < '{{.CutOffDate}}'
    AND commit_time > ('{{.CutOffDate}}'::DATE - INTERVAL '366 day')
)
INSERT INTO TraceValues_trimmer (shard, commit_id)
SELECT DISTINCT shard, t.commit_id
FROM TraceValues t JOIN last_year_commits c ON t.commit_id = c.commit_id
WHERE shard IN (0,1,2,3,4,5,6,7)
LIMIT {{.BatchSize}};

BEGIN;
DELETE FROM TraceValues o
WHERE EXISTS (
  SELECT 1 FROM TraceValues_trimmer t
  WHERE o.shard = t.shard AND o.commit_id = t.commit_id
);
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
