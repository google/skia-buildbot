package sqltracestore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/perf/go/sql/sqltest"
)

func createTraceParamStoreForTests(t *testing.T) *SQLTraceParamStore {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	return NewTraceParamStore(db)
}

func TestWriteRead_Success(t *testing.T) {
	traceStore := createTraceParamStoreForTests(t)
	traceParamMap := map[string]paramtools.Params{
		string(traceIDForSQLFromTraceName(",a=b,c=d,")): {
			"a": "b",
			"c": "d",
		},
	}
	ctx := context.Background()
	err := traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	// Let's try to read now.
	traceParamsFromDb, err := traceStore.ReadParams(ctx, []string{
		string(traceIDForSQLFromTraceName(",a=b,c=d,")),
	})
	assert.NoError(t, err)
	assert.NotNil(t, traceParamsFromDb)
	assert.Equal(t, traceParamMap, traceParamsFromDb)
}

func TestRead_NoRows_Success(t *testing.T) {
	traceStore := createTraceParamStoreForTests(t)
	ctx := context.Background()

	// Let's try to read where no rows exist.
	traceParamsFromDb, err := traceStore.ReadParams(ctx, []string{
		string(traceIDForSQLFromTraceName(",a=b,c=d,")),
	})
	assert.NoError(t, err)
	// Expect an empty map.
	assert.Equal(t, map[string]paramtools.Params{}, traceParamsFromDb)
}
