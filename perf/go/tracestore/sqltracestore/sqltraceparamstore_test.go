package sqltracestore

import (
	"context"
	"fmt"
	"strconv"
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"go.skia.org/infra/go/deepequal/assertdeep"
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

func TestWriteLargeNumber_Success(t *testing.T) {
	traceStore := createTraceParamStoreForTests(t)
	traceParamMap := map[string]paramtools.Params{}
	traceIds := []string{}
	// Make the trace count larger than chunk size to make it run in parallel.
	for i := range 2 * traceParamInsertChunkSize {
		traceId := string(traceIDForSQLFromTraceName(fmt.Sprintf(",num=%d,", i)))
		traceParamMap[traceId] = paramtools.Params{
			"num": strconv.Itoa(i),
		}
		traceIds = append(traceIds, traceId)
	}

	ctx := context.Background()
	err := traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)
	// Let's try to read now.
	traceParamsFromDb, err := traceStore.ReadParams(ctx, traceIds)
	assert.NoError(t, err)
	assertdeep.Equal(t, traceParamMap, traceParamsFromDb)
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

func TestTraceIdFromBytesBackToBytesConversion(t *testing.T) {
	makeId := func(x int) string {
		// nameSpace selection doesn't matter, we just want some IDs generated.
		return uuid.NewSHA1(uuid.NameSpaceOID, []byte(fmt.Sprintf("%x", x))).String()
	}

	testDataLen := 1000
	testData := make([][]byte, testDataLen)
	for i := range testDataLen {
		s := makeId(i)
		testData[i] = []byte(s)
	}

	testDataForSql := make([]string, testDataLen)
	for i := range testDataLen {
		testDataForSql[i] = string(traceIDForSQLFromTraceIDAsBytes(testData[i]))
	}

	testDataResults, err := convertTraceIDsToBytes(testDataForSql)
	assert.NoError(t, err)
	assert.Equal(t, testData, testDataResults)
}
