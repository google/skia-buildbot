package promoter

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/trace_visibility/sqlconfigstore"
	"go.skia.org/infra/perf/go/types"
)

func TestPromote_Success(t *testing.T) {
	ctx := context.Background()

	// 1. Boot up real, in-memory Spanner emulator instance for testing
	db := sqltest.NewSpannerDBForTests(t, "trace_visibility")

	configStore := sqlconfigstore.New(db)
	p := New(db, configStore)

	// 2. Set up the test active visibility rules in DB
	err := configStore.Set(ctx, "bot=builder-a")
	require.NoError(t, err)

	// 3. Seed trace data
	traceName1 := ",benchmark=motionmark,bot=builder-a,"
	traceName2 := ",benchmark=motionmark,bot=builder-b,"
	traceName3 := ",benchmark=jetstream,bot=builder-a,"

	tidBytes1 := types.TraceIDForSQLInBytesFromTraceName(traceName1)
	tidBytes2 := types.TraceIDForSQLInBytesFromTraceName(traceName2)
	tidBytes3 := types.TraceIDForSQLInBytesFromTraceName(traceName3)

	// Seed TraceParams table (all start private)
	_, err = db.Exec(ctx, `
		INSERT INTO TraceParams (trace_id, params, is_public)
		VALUES
			($1, '{"benchmark": "motionmark", "bot": "builder-a"}', false),
			($2, '{"benchmark": "motionmark", "bot": "builder-b"}', false),
			($3, '{"benchmark": "jetstream",  "bot": "builder-a"}', NULL)
	`, tidBytes1[:], tidBytes2[:], tidBytes3[:])
	require.NoError(t, err)

	// 4. Run the promotion sweep!
	promotedCount, err := p.Promote(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, promotedCount, "Expected 2 traces to be promoted")

	// 5. Verify database updates
	var isPublic1 bool
	err = db.QueryRow(ctx, "SELECT is_public FROM TraceParams WHERE trace_id = $1", tidBytes1[:]).Scan(&isPublic1)
	assert.NoError(t, err)
	assert.True(t, isPublic1, "Trace 1 (bot=builder-a) should have been promoted to public")

	var isPublic2 bool
	err = db.QueryRow(ctx, "SELECT is_public FROM TraceParams WHERE trace_id = $1", tidBytes2[:]).Scan(&isPublic2)
	assert.NoError(t, err)
	assert.False(t, isPublic2, "Trace 2 (bot=builder-b) should remain private")

	var isPublic3 bool
	err = db.QueryRow(ctx, "SELECT is_public FROM TraceParams WHERE trace_id = $1", tidBytes3[:]).Scan(&isPublic3)
	assert.NoError(t, err)
	assert.True(t, isPublic3, "Trace 3 (bot=builder-a) should have been promoted to public")
}

func TestPromote_AlreadyPublicTraces_Skipped(t *testing.T) {
	ctx := context.Background()

	db := sqltest.NewSpannerDBForTests(t, "trace_visibility_skipped")
	configStore := sqlconfigstore.New(db)
	p := New(db, configStore)

	err := configStore.Set(ctx, "bot=builder-a")
	require.NoError(t, err)

	traceName1 := ",benchmark=motionmark,bot=builder-a,"
	tidBytes1 := types.TraceIDForSQLInBytesFromTraceName(traceName1)

	// Seed trace as already public
	_, err = db.Exec(ctx, `
		INSERT INTO TraceParams (trace_id, params, is_public)
		VALUES ($1, '{"benchmark": "motionmark", "bot": "builder-a"}', true)
	`, tidBytes1[:])
	require.NoError(t, err)

	// Run sweep. Should exit successfully.
	promotedCount, err := p.Promote(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, promotedCount, "Expected 0 traces to be promoted since they are already public")

	var isPublic bool
	err = db.QueryRow(ctx, "SELECT is_public FROM TraceParams WHERE trace_id = $1", tidBytes1[:]).Scan(&isPublic)
	assert.NoError(t, err)
	assert.True(t, isPublic)
}

func TestBuildPromotionQuery(t *testing.T) {
	rules := []string{
		"bot=builder-a",
		"benchmark=motionmark",
		"invalid-rule",
	}

	// Case 1: No cursor (lastTID is nil)
	query, args := buildPromotionQuery(rules, 100, nil)
	assert.Contains(t, query, "COALESCE(is_public, false) = false")
	assert.Contains(t, query, "params ->> 'bot' = $1")
	assert.Contains(t, query, "params ->> 'benchmark' = $2")
	assert.NotContains(t, query, "trace_id >")
	assert.Contains(t, query, "LIMIT 100")
	assert.Len(t, args, 2)
	assert.Equal(t, "builder-a", args[0])
	assert.Equal(t, "motionmark", args[1])

	// Case 2: With cursor (lastTID is specified)
	lastTID := []byte("my-trace-id")
	queryWithCursor, argsWithCursor := buildPromotionQuery(rules, 100, lastTID)
	assert.Contains(t, queryWithCursor, "params ->> 'bot' = $1")
	assert.Contains(t, queryWithCursor, "params ->> 'benchmark' = $2")
	assert.Contains(t, queryWithCursor, "AND trace_id > $3")
	assert.Len(t, argsWithCursor, 3)
	assert.Equal(t, "builder-a", argsWithCursor[0])
	assert.Equal(t, "motionmark", argsWithCursor[1])
	assert.Equal(t, lastTID, argsWithCursor[2])

	// Empty rules case
	queryEmpty, argsEmpty := buildPromotionQuery(nil, 100, nil)
	assert.Empty(t, queryEmpty)
	assert.Nil(t, argsEmpty)
}
