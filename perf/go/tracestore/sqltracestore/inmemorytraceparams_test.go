package sqltracestore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/perf/go/sql/sqltest"
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

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60)
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
		string(traceIDForSQLFromTraceName(",a=b,c=d,")): {
			"a": "b",
			"c": "d",
		},
		string(traceIDForSQLFromTraceName(",a=notb,c=notd,")): {
			"a": "notb",
			"c": "notd",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60)
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
		string(traceIDForSQLFromTraceName(",a=b,c=d,")): {
			"a": "b",
			"c": "d",
		},
		string(traceIDForSQLFromTraceName(",a=notb,c=notd,")): {
			"a": "notb",
			"c": "notd",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60)
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
		string(traceIDForSQLFromTraceName(",a=b,c=d,")): {
			"a": "b",
			"c": "d",
		},
		string(traceIDForSQLFromTraceName(",a=notb,c=notd,")): {
			"a": "notb",
			"c": "notd",
		},
		string(traceIDForSQLFromTraceName(",a=c,c=d,")): {
			"a": "c",
			"c": "d",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60)
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
		string(traceIDForSQLFromTraceName(",a=b,c=d,")): {
			"a": "b",
			"c": "d",
		},
		string(traceIDForSQLFromTraceName(",a=notb,c=notd,")): {
			"a": "notb",
			"c": "notd",
		},
		string(traceIDForSQLFromTraceName(",a=c,c=d,")): {
			"a": "c",
			"c": "d",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60)
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
		string(traceIDForSQLFromTraceName(",a=b,")): {
			"a": "b",
		},
	}
	err = traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db, 12*60*60)
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
