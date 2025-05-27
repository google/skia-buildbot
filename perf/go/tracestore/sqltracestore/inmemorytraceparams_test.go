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

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db)
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

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db)
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

	inMemoryTraceParams, err := NewInMemoryTraceParams(ctx, db)
	assert.NoError(t, err)

	outParams := make(chan paramtools.Params, 10000)
	q, err := query.NewFromString("a=b&c=notd")
	inMemoryTraceParams.QueryTraceIDs(ctx, 176, q, outParams)
	assert.NoError(t, err)
	queryResult := outParamsToSlice(outParams)
	// Expect no results
	assert.Equal(t, 0, len(queryResult))
}
