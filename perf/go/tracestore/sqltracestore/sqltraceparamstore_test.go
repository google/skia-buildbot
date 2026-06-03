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
	"go.skia.org/infra/perf/go/types"
)

func createTraceParamStoreForTests(t *testing.T) *SQLTraceParamStore {
	db := sqltest.NewSpannerDBForTests(t, "tracestore")
	return NewTraceParamStore(db)
}

func TestWriteRead_Success(t *testing.T) {
	traceStore := createTraceParamStoreForTests(t)
	traceParamMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",a=b,c=d,")): {
			"a": "b",
			"c": "d",
		},
	}
	ctx := context.Background()
	err := traceStore.WriteTraceParams(ctx, traceParamMap)
	assert.NoError(t, err)

	// Let's try to read now.
	traceParamsFromDb, err := traceStore.ReadParams(ctx, []string{
		string(types.TraceIDForSQLFromTraceName(",a=b,c=d,")),
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
		traceId := string(types.TraceIDForSQLFromTraceName(fmt.Sprintf(",num=%d,", i)))
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
		string(types.TraceIDForSQLFromTraceName(",a=b,c=d,")),
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
		testDataForSql[i] = string(types.TraceIDForSQLFromTraceIDAsBytes(testData[i]))
	}

	testDataResults, err := convertTraceIDsToBytes(testDataForSql)
	assert.NoError(t, err)
	assert.Equal(t, testData, testDataResults)
}

func TestGetInternalTraceIDsForParam_Success(t *testing.T) {
	traceStore := createTraceParamStoreForTests(t)
	ctx := context.Background()

	// Write two traces with the same param key "bot" but different values.
	// By default, they will have is_public = NULL (which is treated as false).
	traceIdsMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",bot=public_bot,")): {
			"bot": "public_bot",
		},
		string(types.TraceIDForSQLFromTraceName(",bot=private_bot,")): {
			"bot": "private_bot",
		},
	}
	err := traceStore.WriteTraceParams(ctx, traceIdsMap)
	assert.NoError(t, err)

	// Promote the public_bot trace to is_public = true
	publicId := string(types.TraceIDForSQLFromTraceName(",bot=public_bot,"))
	err = traceStore.UpdateVisibility(ctx, []string{publicId}, true)
	assert.NoError(t, err)

	// Test 1: Query for internal traces using public bot name. Should return nothing (as it is public).
	ids, err := traceStore.GetInternalTraceIDsForParam(ctx, "bot", "public_bot")
	assert.NoError(t, err)
	assert.Empty(t, ids)

	// Test 2: Query for internal traces using private bot name. Should return private_bot trace.
	// This tests that NULL is correctly treated as false by the COALESCE logic.
	privateId := string(types.TraceIDForSQLFromTraceName(",bot=private_bot,"))
	ids, err = traceStore.GetInternalTraceIDsForParam(ctx, "bot", "private_bot")
	assert.NoError(t, err)
	assert.Equal(t, []string{privateId}, ids)
}

func TestGetPublicTraces_Success(t *testing.T) {
	traceStore := createTraceParamStoreForTests(t)
	ctx := context.Background()

	traceIdsMap := map[string]paramtools.Params{
		string(types.TraceIDForSQLFromTraceName(",bot=public_bot,")): {
			"bot": "public_bot",
		},
		string(types.TraceIDForSQLFromTraceName(",bot=private_bot,")): {
			"bot": "private_bot",
		},
	}
	err := traceStore.WriteTraceParams(ctx, traceIdsMap)
	assert.NoError(t, err)

	// Promote one trace to public
	publicId := string(types.TraceIDForSQLFromTraceName(",bot=public_bot,"))
	err = traceStore.UpdateVisibility(ctx, []string{publicId}, true)
	assert.NoError(t, err)

	// Fetch all public traces
	publicTraces, err := traceStore.GetPublicTraces(ctx)
	assert.NoError(t, err)
	assert.Len(t, publicTraces, 1)

	assert.Contains(t, publicTraces, publicId)
	assert.Equal(t, "public_bot", publicTraces[publicId]["bot"])
}
