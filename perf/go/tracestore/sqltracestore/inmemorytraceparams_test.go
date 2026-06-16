package sqltracestore

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/tracestore"
	"go.skia.org/infra/perf/go/types"
)

func outParamsToSlice(outParams chan paramtools.Params) []paramtools.Params {
	ret := []paramtools.Params{}
	for p := range outParams {
		ret = append(ret, p)
	}
	return ret
}

func TestQueryTraceIDs_NoRows_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert a tilenumber and paramset to generate "bot" col from:
	insertIntoParamSets := `
	INSERT INTO
		ParamSets (tile_number, param_key, param_value)
	VALUES
			( 176, 'bot', 'win-10-perf' )
	ON CONFLICT (tile_number, param_key, param_value)
	DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	q, err := query.NewFromString("a=b&c=d")
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)
	assert.NoError(t, err)
	queryResult := outParamsToSlice(outParams)
	// Expect no results
	assert.Equal(t, 0, len(queryResult))
}

func TestQueryTraceIDs_SimpleQueryOneResult_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets to generate "a" and "c" cols from:
	insertIntoParamSets := `
	INSERT INTO
		ParamSets (tile_number, param_key, param_value)
	VALUES
			( 176, 'a', 'b' ),
			( 176, 'a', 'notb' ),
			( 176, 'c', 'd' ),
			( 176, 'c', 'notd' )
	ON CONFLICT (tile_number, param_key, param_value)
	DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=b,c=d,")): {
			"a": "b",
			"c": "d",
		},
		string(types.TraceIDForSQLFromTraceName(",a=notb,c=notd,")): {
			"a": "notb",
			"c": "notd",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	q, err := query.NewFromString("a=b&c=d")
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)
	assert.NoError(t, err)
	queryResult := outParamsToSlice(outParams)
	// Expect one result
	assert.Equal(t, 1, len(queryResult))
}

func TestQueryTraceIDs_SimpleQueryNoMatches_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets to generate "a" and "c" cols from:
	insertIntoParamSets := `
	INSERT INTO
		ParamSets (tile_number, param_key, param_value)
	VALUES
			( 176, 'a', 'b' ),
			( 176, 'a', 'notb' ),
			( 176, 'c', 'd' ),
			( 176, 'c', 'notd' )
	ON CONFLICT (tile_number, param_key, param_value)
	DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=b,c=d,")): {
			"a": "b",
			"c": "d",
		},
		string(types.TraceIDForSQLFromTraceName(",a=notb,c=notd,")): {
			"a": "notb",
			"c": "notd",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	q, err := query.NewFromString("a=b&c=notd")
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)
	assert.NoError(t, err)
	queryResult := outParamsToSlice(outParams)
	// Expect no results
	assert.Equal(t, 0, len(queryResult))
}

func TestQueryTraceIDs_NegativeMatch_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets to generate "a" and "c" cols from:
	insertIntoParamSets := `
    INSERT INTO
        ParamSets (tile_number, param_key, param_value)
    VALUES
            ( 176, 'a', 'b' ),
            ( 176, 'a', 'notb' ),
            ( 176, 'c', 'd' ),
            ( 176, 'c', 'notd' )
    ON CONFLICT (tile_number, param_key, param_value)
    DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=b,c=d,")): {
			"a": "b",
			"c": "d",
		},
		string(types.TraceIDForSQLFromTraceName(",a=notb,c=notd,")): {
			"a": "notb",
			"c": "notd",
		},
		string(types.TraceIDForSQLFromTraceName(",a=c,c=d,")): {
			"a": "c",
			"c": "d",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	// a!=b
	q, err := query.NewFromString("a=!b")
	assert.NoError(t, err)
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)

	queryResult := outParamsToSlice(outParams)
	// Expect two results: a=notb and a=c
	assert.Equal(t, 2, len(queryResult))
}

func TestQueryTraceIDs_NegativeRegexMatch_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets
	insertIntoParamSets := `
    INSERT INTO
        ParamSets (tile_number, param_key, param_value)
    VALUES
            ( 176, 'a', 'b' ),
            ( 176, 'a', 'notb' ),
            ( 176, 'c', 'd' ),
            ( 176, 'c', 'notd' )
    ON CONFLICT (tile_number, param_key, param_value)
    DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=b,c=d,")): {
			"a": "b",
			"c": "d",
		},
		string(types.TraceIDForSQLFromTraceName(",a=notb,c=notd,")): {
			"a": "notb",
			"c": "notd",
		},
		string(types.TraceIDForSQLFromTraceName(",a=c,c=d,")): {
			"a": "c",
			"c": "d",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	// a matches anything that does not start with n
	q, err := query.NewFromString("a=!~^n.*")
	assert.NoError(t, err)
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)

	queryResult := outParamsToSlice(outParams)
	// Expect two results: a=b and a=c
	assert.Equal(t, 2, len(queryResult))
}

func TestQueryTraceIDs_NonExistentKey_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets
	insertIntoParamSets := `
    INSERT INTO
        ParamSets (tile_number, param_key, param_value)
    VALUES
            ( 176, 'a', 'b' )
    ON CONFLICT (tile_number, param_key, param_value)
    DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=b,")): {
			"a": "b",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	// z is not a valid key
	q, err := query.NewFromString("z=y")
	assert.NoError(t, err)
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)

	queryResult := outParamsToSlice(outParams)
	// Expect 0 results
	assert.Equal(t, 0, len(queryResult))
}

func TestQueryTraceIDs_KeyOnlyInOlderTile_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets: 'arch' only in tile 174, 'config' in tile 176.
	insertIntoParamSets := `
    INSERT INTO
        ParamSets (tile_number, param_key, param_value)
    VALUES
            ( 174, 'arch', 'linux32' ),
            ( 176, 'config', '8888' )
    ON CONFLICT (tile_number, param_key, param_value)
    DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",arch=linux32,")): {
			"arch": "linux32",
		},
		string(types.TraceIDForSQLFromTraceName(",config=8888,")): {
			"config": "8888",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	q, err := query.NewFromString("arch=linux32")
	assert.NoError(t, err)
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)

	queryResult := outParamsToSlice(outParams)
	// Expect 0 results because keys only in older tiles are not indexed by design.
	assert.Equal(t, 0, len(queryResult))
}

func TestQueryTraceIDs_PositiveRegexMatch_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets
	insertIntoParamSets := `
    INSERT INTO
        ParamSets (tile_number, param_key, param_value)
    VALUES
            ( 176, 'a', 'apple' ),
            ( 176, 'a', 'banana' ),
            ( 176, 'a', 'apricot' )
    ON CONFLICT (tile_number, param_key, param_value)
    DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=apple,")): {
			"a": "apple",
		},
		string(types.TraceIDForSQLFromTraceName(",a=banana,")): {
			"a": "banana",
		},
		string(types.TraceIDForSQLFromTraceName(",a=apricot,")): {
			"a": "apricot",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	// a matches anything starting with ap
	q, err := query.NewFromString("a=~^ap.*")
	assert.NoError(t, err)
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)

	queryResult := outParamsToSlice(outParams)
	// Expect two results: apple and apricot
	assert.Equal(t, 2, len(queryResult))
}

func TestQueryTraceIDs_EmptyQuery_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets
	insertIntoParamSets := `
    INSERT INTO
        ParamSets (tile_number, param_key, param_value)
    VALUES
            ( 176, 'a', 'b' ),
            ( 176, 'c', 'd' )
    ON CONFLICT (tile_number, param_key, param_value)
    DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=b,")): {
			"a": "b",
		},
		string(types.TraceIDForSQLFromTraceName(",c=d,")): {
			"c": "d",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	q := &query.Query{} // Empty query
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)

	queryResult := outParamsToSlice(outParams)
	// Expect all results (2)
	assert.Equal(t, 2, len(queryResult))
}

func TestQueryTraceIDs_IntersectionEmpty_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets
	insertIntoParamSets := `
    INSERT INTO
        ParamSets (tile_number, param_key, param_value)
    VALUES
            ( 176, 'a', 'b' ),
            ( 176, 'c', 'd' )
    ON CONFLICT (tile_number, param_key, param_value)
    DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=b,")): {
			"a": "b",
		},
		string(types.TraceIDForSQLFromTraceName(",c=d,")): {
			"c": "d",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	q, err := query.NewFromString("a=b&c=d")
	assert.NoError(t, err)
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)

	queryResult := outParamsToSlice(outParams)
	// Expect 0 results because no trace has BOTH a=b and c=d
	assert.Equal(t, 0, len(queryResult))
}

func TestQueryTraceIDs_OnlyNegativeMatch_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets
	insertIntoParamSets := `
    INSERT INTO
        ParamSets (tile_number, param_key, param_value)
    VALUES
            ( 176, 'a', 'b' ),
            ( 176, 'a', 'c' )
    ON CONFLICT (tile_number, param_key, param_value)
    DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=b,")): {
			"a": "b",
		},
		string(types.TraceIDForSQLFromTraceName(",a=c,")): {
			"a": "c",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	// a!=b
	q, err := query.NewFromString("a=!b")
	assert.NoError(t, err)
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)

	queryResult := outParamsToSlice(outParams)
	// Expect one result: a=c
	assert.Equal(t, 1, len(queryResult))
	assert.Equal(t, "c", queryResult[0]["a"])
}

func TestQueryTraceIDs_OrLogic_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets
	insertIntoParamSets := `
    INSERT INTO
        ParamSets (tile_number, param_key, param_value)
    VALUES
            ( 176, 'a', 'b' ),
            ( 176, 'a', 'c' ),
            ( 176, 'a', 'd' )
    ON CONFLICT (tile_number, param_key, param_value)
    DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=b,")): {
			"a": "b",
		},
		string(types.TraceIDForSQLFromTraceName(",a=c,")): {
			"a": "c",
		},
		string(types.TraceIDForSQLFromTraceName(",a=d,")): {
			"a": "d",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	// a=b or a=c
	q, err := query.NewFromString("a=b&a=c")
	assert.NoError(t, err)
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)

	queryResult := outParamsToSlice(outParams)
	// Expect two results: a=b and a=c
	assert.Equal(t, 2, len(queryResult))
}

func TestQueryTraceIDs_MultipleNegativeMatch_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets
	insertIntoParamSets := `
    INSERT INTO
        ParamSets (tile_number, param_key, param_value)
    VALUES
            ( 176, 'a', 'b' ),
            ( 176, 'a', 'c' ),
            ( 176, 'a', 'd' )
    ON CONFLICT (tile_number, param_key, param_value)
    DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=b,")): {
			"a": "b",
		},
		string(types.TraceIDForSQLFromTraceName(",a=c,")): {
			"a": "c",
		},
		string(types.TraceIDForSQLFromTraceName(",a=d,")): {
			"a": "d",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	// a!=b and a!=c
	q, err := query.NewFromString("a=!b&a=!c")
	assert.NoError(t, err)
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)

	queryResult := outParamsToSlice(outParams)
	// Expect one result: a=d
	assert.Equal(t, 1, len(queryResult))
	assert.Equal(t, "d", queryResult[0]["a"])
}

func TestInMemoryTraceParams_ShowOnlyPublicTraces_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets to cover key/values in the index:
	insertIntoParamSets := `
	INSERT INTO
		ParamSets (tile_number, param_key, param_value)
	VALUES
			( 176, 'a', 'b' ),
			( 176, 'a', 'notb' )
	ON CONFLICT (tile_number, param_key, param_value)
	DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	publicTraceName := ",a=b,"
	privateTraceName := ",a=notb,"
	publicTraceID := types.TraceIDForSQLFromTraceName(publicTraceName)
	privateTraceID := types.TraceIDForSQLFromTraceName(privateTraceName)

	traceParamMap := map[string]paramtools.Params{
		string(publicTraceID): {
			"a": "b",
		},
		string(privateTraceID): {
			"a": "notb",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	// Since WriteTraceParams does not handle is_public, we manually execute raw SQL updates.
	publicBytes := types.TraceIDForSQLInBytesFromTraceName(publicTraceName)
	_, err = db.Exec(ctx, "UPDATE TraceParams SET is_public = TRUE WHERE trace_id = $1", publicBytes[:])
	assert.NoError(t, err)

	// Instantiate InMemoryTraceParams with showOnlyPublicTraces = true
	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, true)
	assert.NoError(t, err)

	// Verify TraceAccessAllowed behaviors
	assert.True(t, inMemoryTraceParams.TraceAccessAllowed(publicTraceName))
	assert.False(t, inMemoryTraceParams.TraceAccessAllowed(privateTraceName))
	assert.False(t, inMemoryTraceParams.TraceAccessAllowed(",a=nonexistent,"))

	// Verify Query results exclude the private trace
	outParams := make(chan paramtools.Params, 10)
	q := &query.Query{}
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)
	queryResult := outParamsToSlice(outParams)
	assert.Equal(t, 1, len(queryResult))
	assert.Equal(t, "b", queryResult[0]["a"])
}

func TestInMemoryTraceParams_GetParamSet_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	traceStore := NewTraceParamStore(db)
	publicTraceName := ",a=b,c=d,"
	privateTraceName := ",a=notb,c=notd,"
	publicTraceID := types.TraceIDForSQLFromTraceName(publicTraceName)
	privateTraceID := types.TraceIDForSQLFromTraceName(privateTraceName)

	traceParamMap := map[string]paramtools.Params{
		string(publicTraceID): {
			"a": "b",
			"c": "d",
		},
		string(privateTraceID): {
			"a": "notb",
			"c": "notd",
		},
	}
	err := traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	// Since WriteTraceParams does not handle is_public, we manually execute raw SQL updates.
	publicBytes := types.TraceIDForSQLInBytesFromTraceName(publicTraceName)
	_, err = db.Exec(ctx, "UPDATE TraceParams SET is_public = TRUE WHERE trace_id = $1", publicBytes[:])
	assert.NoError(t, err)

	// Let's insert the ParamSets record to ensure paramCols are generated correctly
	insertIntoParamSets := `
	INSERT INTO
		ParamSets (tile_number, param_key, param_value)
	VALUES
			( 176, 'a', 'b' ),
			( 176, 'a', 'notb' ),
			( 176, 'c', 'd' ),
			( 176, 'c', 'notd' )
	ON CONFLICT (tile_number, param_key, param_value)
	DO NOTHING`
	_, err = db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	// Instantiate InMemoryTraceParams with showOnlyPublicTraces = true
	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, true)
	assert.NoError(t, err)

	// Check the public in-memory parameter set output
	ps := inMemoryTraceParams.GetParamSet()
	expected := paramtools.ParamSet{
		"a": []string{"b"},
		"c": []string{"d"},
	}
	assert.Equal(t, expected.Freeze(), ps)
}

func TestGetWasmCache_Success(t *testing.T) {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	ctx := context.Background()

	// Insert paramsets
	insertIntoParamSets := `
	INSERT INTO
		ParamSets (tile_number, param_key, param_value)
	VALUES
			( 176, 'a', 'b' ),
			( 176, 'a', 'notb' ),
			( 176, 'c', 'd' ),
			( 176, 'common', 'val' )
	ON CONFLICT (tile_number, param_key, param_value)
	DO NOTHING`
	_, err := db.Exec(ctx, insertIntoParamSets)
	assert.NoError(t, err)

	traceStore := NewTraceParamStore(db)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=b,c=d,common=val,")): {
			"a":      "b",
			"c":      "d",
			"common": "val",
		},
		string(types.TraceIDForSQLFromTraceName(",a=notb,common=val,")): {
			"a":      "notb",
			"common": "val",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60, false)
	assert.NoError(t, err)

	ps := paramtools.ParamSet{
		"a":      []string{"b", "notb"},
		"c":      []string{"d"},
		"common": []string{"val"},
	}

	cacheData, err := inMemoryTraceParams.GetWasmCache(ctx, ps.Freeze())
	assert.NoError(t, err)
	assert.NotNil(t, cacheData)

	// Verify Meta
	var meta struct {
		Stride       int               `json:"stride"`
		Count        int               `json:"count"`
		Version      string            `json:"version"`
		CommonParams map[string]string `json:"commonParams"`
	}
	err = json.Unmarshal(cacheData.Meta, &meta)
	assert.NoError(t, err)
	assert.Equal(t, 2, meta.Count)
	assert.Equal(t, "val", meta.CommonParams["common"])
	_, ok := meta.CommonParams["a"]
	assert.False(t, ok)
	// Stride should be len(filteredPs) rounded to multiple of 8.
	// filteredPs has "a" and "c". len is 2. Stride should be 8.
	assert.Equal(t, 8, meta.Stride)

	// Verify Params
	var params []tracestore.WasmParam
	err = json.Unmarshal(cacheData.Params, &params)
	assert.NoError(t, err)
	// Should contain "a" (b, notb) and "c" (d). "common" should be excluded.
	assert.Equal(t, 3, len(params))

	// Verify they are sorted/consistent
	assert.Equal(t, "a", params[0].Key)
	assert.Equal(t, "b", params[0].Value)
	assert.Equal(t, "a", params[1].Key)
	assert.Equal(t, "notb", params[1].Value)
	assert.Equal(t, "c", params[2].Key)
	assert.Equal(t, "d", params[2].Value)

	assert.Equal(t, 32, len(cacheData.Traces))

	// Expected content:
	// Row 1: [1, 3, 0, 0, 0, 0, 0, 0]
	// Row 2: [2, 0, 0, 0, 0, 0, 0, 0]
	expectedTraces := make([]byte, 32)
	// Row 1
	expectedTraces[0], expectedTraces[1] = 1, 0
	expectedTraces[2], expectedTraces[3] = 3, 0
	// Row 2
	expectedTraces[16], expectedTraces[17] = 2, 0

	assert.Equal(t, expectedTraces, cacheData.Traces)
}
