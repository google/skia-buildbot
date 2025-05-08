package expectedschema

import (
	"context"
	"time"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/go/sql/schema"
	"go.skia.org/infra/perf/go/types"
)

// Timeout used on Contexts when making SQL requests.
const sqlTimeout = time.Minute

const getLatestTile = `
SELECT
	tile_number
FROM
	paramsets
ORDER BY
	tile_number DESC
LIMIT
	1;
`

const paramSetForTile = `
SELECT
	DISTINCT param_key
FROM
	paramsets
WHERE
	tile_number = $1 OR tile_number = $1-1;
`

// Gets lists of _generated_ columns and indexes in the traceparams table.
// This means columns other than "trace_id", "params", and "createdat", and
// indexes other than "PRIMARY_KEY".
func GetTraceParamsGeneratedColsAndIdxs(ctx context.Context, db pool.Pool, databaseType string) ([]string, []string, error) {
	ctx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()
	columnNames := []string{}
	indexNames := []string{}
	if databaseType != schema.SpannerDBType {
		return columnNames, indexNames, nil
	}

	rows, err := db.Query(ctx, schema.TypesQuerySpanner, "traceparams")
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	for rows.Next() {
		var colName string
		var colType string
		err := rows.Scan(&colName, &colType)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		if colName == "trace_id" || colName == "params" || colName == "createdat" {
			continue
		}
		columnNames = append(columnNames, colName)
	}

	rows, err = db.Query(ctx, schema.SpannerIndexNameQuery, "traceparams")
	if err != nil {
		return nil, nil, skerr.Wrap(err)
	}
	for rows.Next() {
		var indexName string
		err := rows.Scan(&indexName)
		if err != nil {
			return nil, nil, skerr.Wrap(err)
		}
		if indexName == "PRIMARY_KEY" {
			continue
		}
		indexNames = append(indexNames, indexName)
	}

	return columnNames, indexNames, nil
}

// Gets a list of the param keys that are in use in the last two tiles of this database.
func GetParams(ctx context.Context, db pool.Pool, databaseType string) ([]string, error) {
	ctx, cancel := context.WithTimeout(ctx, sqlTimeout)
	defer cancel()
	var params []string
	if databaseType != schema.SpannerDBType {
		return params, nil
	}

	tileNumber := types.BadTileNumber
	if err := db.QueryRow(ctx, getLatestTile).Scan(&tileNumber); err != nil {
		if err == pgx.ErrNoRows {
			sklog.Warning("Querying for latest tile in schema.GetParams returned no rows!")
			return params, nil
		}
		return nil, skerr.Wrap(err)
	}
	rows, err := db.Query(ctx, paramSetForTile, tileNumber)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, skerr.Wrapf(err, "Failed scanning row - tileNumber=%d", tileNumber)
		}
		params = append(params, key)
	}

	return params, nil
}
