package common

import (
	"context"

	"github.com/jackc/pgx/v4/pgxpool"
	"go.opencensus.io/trace"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/golden/go/search/query"
	"go.skia.org/infra/golden/go/sql/schema"
)

// To avoid piping a lot of info about the commits in the most recent window through all the
// functions in the search pipeline, we attach them as values to the context.
type SearchContextKey string

const (
	ActualWindowLengthKey = SearchContextKey("actualWindowLengthKey")
	CommitToIdxKey        = SearchContextKey("commitToIdxKey")
	FirstCommitIDKey      = SearchContextKey("firstCommitIDKey")
	FirstTileIDKey        = SearchContextKey("firstTileIDKey")
	LastTileIDKey         = SearchContextKey("lastTileIDKey")
	QualifiedCLIDKey      = SearchContextKey("qualifiedCLIDKey")
	QualifiedPSIDKey      = SearchContextKey("qualifiedPSIDKey")
	QueryKey              = SearchContextKey("queryKey")
)

// addCommitsData finds the current sliding window of data (The last N commits) and adds the
// derived data to the given context and returns it.
func AddCommitsData(ctx context.Context, db *pgxpool.Pool, windowLength int) (context.Context, error) {
	// Note: need to rename the context here to avoid adding the span data to all other contexts.
	sCtx, span := trace.StartSpan(ctx, "addCommitsData")
	defer span.End()
	const statement = `SELECT commit_id, tile_id FROM
CommitsWithData ORDER BY commit_id DESC LIMIT $1`
	rows, err := db.Query(sCtx, statement, windowLength)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	defer rows.Close()
	ids := make([]schema.CommitID, 0, windowLength)
	lastObservedTile := schema.TileID(-1)
	var firstObservedTile schema.TileID
	for rows.Next() {
		var id schema.CommitID
		if err := rows.Scan(&id, &firstObservedTile); err != nil {
			return nil, skerr.Wrap(err)
		}
		if lastObservedTile < firstObservedTile {
			lastObservedTile = firstObservedTile
		}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return nil, skerr.Fmt("No commits with data")
	}
	// ids is ordered most recent commit to last commit at this point
	ctx = context.WithValue(ctx, ActualWindowLengthKey, len(ids))
	ctx = context.WithValue(ctx, FirstCommitIDKey, ids[len(ids)-1])
	ctx = context.WithValue(ctx, FirstTileIDKey, firstObservedTile)
	ctx = context.WithValue(ctx, LastTileIDKey, lastObservedTile)
	idToIndex := map[schema.CommitID]int{}
	idx := 0
	for i := len(ids) - 1; i >= 0; i-- {
		idToIndex[ids[i]] = idx
		idx++
	}
	ctx = context.WithValue(ctx, CommitToIdxKey, idToIndex)
	return ctx, nil
}

func GetFirstCommitID(ctx context.Context) schema.CommitID {
	return ctx.Value(FirstCommitIDKey).(schema.CommitID)
}

func GetFirstTileID(ctx context.Context) schema.TileID {
	return ctx.Value(FirstTileIDKey).(schema.TileID)
}

func GetLastTileID(ctx context.Context) schema.TileID {
	return ctx.Value(LastTileIDKey).(schema.TileID)
}

func GetCommitToIdxMap(ctx context.Context) map[schema.CommitID]int {
	return ctx.Value(CommitToIdxKey).(map[schema.CommitID]int)
}

func GetActualWindowLength(ctx context.Context) int {
	return ctx.Value(ActualWindowLengthKey).(int)
}

func GetQuery(ctx context.Context) query.Search {
	return ctx.Value(QueryKey).(query.Search)
}

func GetQualifiedCL(ctx context.Context) string {
	v := ctx.Value(QualifiedCLIDKey)
	if v == nil {
		return "" // This allows us to use getQualifiedCL as "Is the data for a CL or not?"
	}
	return v.(string)
}

func GetQualifiedPS(ctx context.Context) string {
	return ctx.Value(QualifiedPSIDKey).(string)
}
