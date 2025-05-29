// This file contains useful logic for maintenance tasks to migrate new schema
// changes.

package expectedschema

import (
	"context"
	"fmt"
	"strings"

	"go.skia.org/infra/go/deepequal/assertdeep"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/golden/go/config"
	golden_schema "go.skia.org/infra/golden/go/sql/schema"
)

// The two vars below should be updated everytime there's a schema change:
//   - FromLiveToNext tells the SQL to execute to apply the change
//   - FromNextToLive tells the SQL to revert the change
//
// Also we need to update LiveSchema schema and DropTables in sql_test.go:
//   - DropTables deletes all tables *including* the new one in the change.
//   - LiveSchema creates all existing tables *without* the new one in the
//     change.
//
// DO NOT DROP TABLES IN VAR BELOW.
// FOR MODIFYING COLUMNS USE ADD/DROP COLUMN INSTEAD.
var FromLiveToNext = `ALTER TABLE Changelists ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE CommitsWithData ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE DiffMetrics ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE ExpectationDeltas ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE ExpectationRecords ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE Expectations ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE GitCommits ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE Groupings ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE IgnoreRules ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE MetadataCommits ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE Options ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE Patchsets ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE PrimaryBranchDiffCalculationWork ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE PrimaryBranchParams ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE ProblemImages ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE SecondaryBranchDiffCalculationWork ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE SecondaryBranchExpectations ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE SecondaryBranchParams ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE SecondaryBranchValues ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE SourceFiles ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE TiledTraceDigests ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE TraceValues ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE Traces ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE TrackingCommits ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE Tryjobs ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE ValuesAtHead ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE DeprecatedIngestedFiles ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
ALTER TABLE DeprecatedExpectationUndos ADD COLUMN createdat TIMESTAMPTZ DEFAULT CURRENT_TIMESTAMP;
`

// Same as above, but will be used when doing schema migration for spanner databases.
// Some statements can be different for CDB v/s Spanner, hence splitting into
// separate variables.
var FromLiveToNextSpanner = ``

// ONLY DROP TABLE IF YOU JUST CREATED A NEW TABLE.
// FOR MODIFYING COLUMNS USE ADD/DROP COLUMN INSTEAD.
var FromNextToLive = `ALTER TABLE Changelists DROP COLUMN createdat;
ALTER TABLE CommitsWithData DROP COLUMN createdat;
ALTER TABLE DiffMetrics DROP COLUMN createdat;
ALTER TABLE ExpectationDeltas DROP COLUMN createdat;
ALTER TABLE ExpectationRecords DROP COLUMN createdat;
ALTER TABLE Expectations DROP COLUMN createdat;
ALTER TABLE GitCommits DROP COLUMN createdat;
ALTER TABLE Groupings DROP COLUMN createdat;
ALTER TABLE IgnoreRules DROP COLUMN createdat;
ALTER TABLE MetadataCommits DROP COLUMN createdat;
ALTER TABLE Options DROP COLUMN createdat;
ALTER TABLE Patchsets DROP COLUMN createdat;
ALTER TABLE PrimaryBranchDiffCalculationWork DROP COLUMN createdat;
ALTER TABLE PrimaryBranchParams DROP COLUMN createdat;
ALTER TABLE ProblemImages DROP COLUMN createdat;
ALTER TABLE SecondaryBranchDiffCalculationWork DROP COLUMN createdat;
ALTER TABLE SecondaryBranchExpectations DROP COLUMN createdat;
ALTER TABLE SecondaryBranchParams DROP COLUMN createdat;
ALTER TABLE SecondaryBranchValues DROP COLUMN createdat;
ALTER TABLE SourceFiles DROP COLUMN createdat;
ALTER TABLE TiledTraceDigests DROP COLUMN createdat;
ALTER TABLE TraceValues DROP COLUMN createdat;
ALTER TABLE Traces DROP COLUMN createdat;
ALTER TABLE TrackingCommits DROP COLUMN createdat;
ALTER TABLE Tryjobs DROP COLUMN createdat;
ALTER TABLE ValuesAtHead DROP COLUMN createdat;
ALTER TABLE DeprecatedIngestedFiles DROP COLUMN createdat;
ALTER TABLE DeprecatedExpectationUndos DROP COLUMN createdat;
`

// ONLY DROP TABLE IF YOU JUST CREATED A NEW TABLE.
// FOR MODIFYING COLUMNS USE ADD/DROP COLUMN INSTEAD.
var FromNextToLiveSpanner = `
`

// This function will check whether there's a new schema checked-in,
// and if so, migrate the schema in the given CockroachDB instance.
func ValidateAndMigrateNewSchema(ctx context.Context, db pool.Pool, datastoreType config.DatabaseType) error {
	sklog.Debugf("Starting validate and migrate. DatastoreType: %s", datastoreType)
	next, err := Load(datastoreType)
	if err != nil {
		return skerr.Wrap(err)
	}
	sklog.Debugf("Next schema: %s", next)
	prev, err := LoadPrev(datastoreType)
	if err != nil {
		return skerr.Wrap(err)
	}
	sklog.Debugf("Prev schema: %s", prev)
	actual, err := schema.GetDescription(ctx, db, golden_schema.Tables{}, string(datastoreType))
	if err != nil {
		return skerr.Wrap(err)
	}
	// Remove expire_at columns from actual schema because they are not defined
	// via regular schema, but are present in the prod database. See
	// removeExpireAtColumns for more details.
	actual, err = removeExpireAtColumns(actual)
	if err != nil {
		return skerr.Wrap(err)
	}
	diffPrevActual := assertdeep.Diff(prev, *actual)
	diffNextActual := assertdeep.Diff(next, *actual)
	sklog.Debugf("Diff prev vs actual: %s", diffPrevActual)
	sklog.Debugf("Diff next vs actual: %s", diffNextActual)

	if diffNextActual != "" && diffPrevActual == "" {
		sklog.Debugf("Next is different from live schema. Will migrate. diffNextActual: %s", diffNextActual)
		fromLiveToNextStmt := FromLiveToNext
		if datastoreType == config.Spanner {
			fromLiveToNextStmt = FromLiveToNextSpanner
		}
		_, err = db.Exec(ctx, fromLiveToNextStmt)
		if err != nil {
			sklog.Errorf("Failed to migrate Schema from prev to next. Prev: %s, Next: %s.", prev, next)
			return skerr.Wrapf(err, "Failed to migrate Schema")
		}

	} else if diffNextActual != "" && diffPrevActual != "" {
		sklog.Errorf("Live schema doesn't match next or previous checked-in schema. diffNextActual: %s, diffPrevActual: %s.", diffNextActual, diffPrevActual)
		return skerr.Fmt("Live schema doesn't match next or previous checked-in schema.")
	}

	return nil
}

// This method iterates over the ColumnNameAndType map and removes all entries
// where the column name ends with ".expire_at". This is done because all
// columns named "expire_at" are not defined via regular schema, but are
// added orthogonally by data retention policy sqls.
func removeExpireAtColumns(inputDesc *schema.Description) (*schema.Description, error) {
	outputDesc := &schema.Description{
		ColumnNameAndType: make(map[string]string),
		IndexNames:        make([]string, len(inputDesc.IndexNames)),
	}
	copy(outputDesc.IndexNames, inputDesc.IndexNames)
	for key, value := range inputDesc.ColumnNameAndType {
		parts := strings.SplitN(key, ".", 2) // Split into at most 2 parts
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return &schema.Description{}, fmt.Errorf("invalid key format: %q in ColumnNameAndType. Key must be in 'a.b' format with non-empty parts", key)
		}
		columnNamePart := parts[1]
		if columnNamePart == "expire_at" {
			continue
		}
		outputDesc.ColumnNameAndType[key] = value
	}
	return outputDesc, nil
}
