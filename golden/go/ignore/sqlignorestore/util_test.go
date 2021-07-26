package sqlignorestore

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"strconv"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/sql/databuilder"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestConvertIgnoreRules_Success(t *testing.T) {
	unittest.SmallTest(t)

	condition, args := ConvertIgnoreRules(nil)
	assert.Equal(t, "false", condition)
	assert.Empty(t, args)

	condition, args = ConvertIgnoreRules([]paramtools.ParamSet{
		{
			"key1": []string{"alpha"},
		},
	})
	assert.Equal(t, `((COALESCE(keys ->> $1::STRING IN ($2), FALSE)))`, condition)
	assert.Equal(t, []interface{}{"key1", "alpha"}, args)

	condition, args = ConvertIgnoreRules([]paramtools.ParamSet{
		{
			"key1": []string{"alpha", "beta"},
			"key2": []string{"gamma"},
		},
		{
			"key3": []string{"delta", "epsilon", "zeta"},
		},
	})
	const expectedCondition = `((COALESCE(keys ->> $1::STRING IN ($2, $3), FALSE) AND COALESCE(keys ->> $4::STRING IN ($5), FALSE))
OR (COALESCE(keys ->> $6::STRING IN ($7, $8, $9), FALSE)))`
	assert.Equal(t, expectedCondition, condition)
	assert.Equal(t, []interface{}{"key1", "alpha", "beta", "key2", "gamma", "key3", "delta", "epsilon", "zeta"}, args)
}

func TestUpdateIgnoredTraces_StartsNull_SetToCorrectValue(t *testing.T) {
	unittest.LargeTest(t)
	existingData := dks.Build()
	// Force these all to be null
	for i := range existingData.Traces {
		existingData.Traces[i].MatchesAnyIgnoreRule = schema.NBNull
	}
	for i := range existingData.ValuesAtHead {
		existingData.ValuesAtHead[i].MatchesAnyIgnoreRule = schema.NBNull
	}
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	// Verify things are null
	row := db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule IS NULL`)
	var count int
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 41, count)
	assert.Equal(t, 41, len(existingData.Traces))
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule IS NULL`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 33, count)
	assert.Equal(t, 33, len(existingData.ValuesAtHead))

	require.NoError(t, UpdateIgnoredTraces(ctx, db))
	// Verify things are not null
	row = db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule IS NULL`)
	count = -1
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 0, count)
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule IS NULL`)
	count = -1
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 0, count)

	expectedData := dks.Build()
	actualTraces := sqltest.GetAllRows(ctx, t, db, "Traces", &schema.TraceRow{}).([]schema.TraceRow)
	assert.ElementsMatch(t, expectedData.Traces, actualTraces)

	actualValuesAtHead := sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}).([]schema.ValueAtHeadRow)
	assert.ElementsMatch(t, expectedData.ValuesAtHead, actualValuesAtHead)
}

func TestUpdateIgnoredTraces_StartsNotNull_UpdatedToCorrectValues(t *testing.T) {
	unittest.LargeTest(t)
	existingData := dks.Build()
	// Force these all to be a sentinel value
	for i := range existingData.Traces {
		existingData.Traces[i].MatchesAnyIgnoreRule = schema.NBFalse
	}
	for i := range existingData.ValuesAtHead {
		existingData.ValuesAtHead[i].MatchesAnyIgnoreRule = schema.NBFalse
	}
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	// Verify things are that sentinel value
	row := db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule = FALSE`)
	var count int
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 41, count)
	assert.Equal(t, 41, len(existingData.Traces))
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule = FALSE`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 33, count)
	assert.Equal(t, 33, len(existingData.ValuesAtHead))

	require.NoError(t, UpdateIgnoredTraces(ctx, db))

	// Verify things were updated to the correct value
	expectedData := dks.Build()
	actualTraces := sqltest.GetAllRows(ctx, t, db, "Traces", &schema.TraceRow{}).([]schema.TraceRow)
	assert.ElementsMatch(t, expectedData.Traces, actualTraces)

	actualValuesAtHead := sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}).([]schema.ValueAtHeadRow)
	assert.ElementsMatch(t, expectedData.ValuesAtHead, actualValuesAtHead)
}

func TestUpdateIgnoredTraces_PartiallySet_UpdatedToCorrectValues(t *testing.T) {
	unittest.LargeTest(t)
	existingData := dks.Build()
	// Only ValuesAtHead are incorrectly set, but should be updated anyway
	for i := range existingData.ValuesAtHead {
		existingData.ValuesAtHead[i].MatchesAnyIgnoreRule = schema.NBFalse
	}
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	// Verify things are that sentinel value
	row := db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule = FALSE`)
	var count int
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 33, count)
	assert.Equal(t, 33, len(existingData.ValuesAtHead))

	require.NoError(t, UpdateIgnoredTraces(ctx, db))

	// Verify things were updated to the correct value
	expectedData := dks.Build()
	actualTraces := sqltest.GetAllRows(ctx, t, db, "Traces", &schema.TraceRow{}).([]schema.TraceRow)
	assert.ElementsMatch(t, expectedData.Traces, actualTraces)

	actualValuesAtHead := sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}).([]schema.ValueAtHeadRow)
	assert.ElementsMatch(t, expectedData.ValuesAtHead, actualValuesAtHead)
}

func TestUpdateIgnoredTraces_MultipleBatches_Success(t *testing.T) {
	unittest.LargeTest(t)

	const numTraces = 6060 // This is a number bigger than the batch size.
	traceIDs := makeRandomTraceIDs(numTraces)
	g := md5.Sum([]byte("whatever grouping"))
	arbitraryBytes := g[:]
	traces := make([]schema.TraceRow, 0, numTraces)
	atHead := make([]schema.ValueAtHeadRow, 0, numTraces)
	// We'll set 1/3 of the total traces and ValuesAtHead to be ignored (based on the keys
	// and the one ignore rule). That way we can make sure the rules were correctly
	// applied.
	i := 0
	for ; i < numTraces/3; i++ {
		traces = append(traces, schema.TraceRow{
			TraceID:    traceIDs[i],
			GroupingID: arbitraryBytes,
			Keys: paramtools.Params{
				types.CorpusField:   "corpus",
				"should_be_ignored": "true",
			},
			MatchesAnyIgnoreRule: schema.NBNull,
		})
		atHead = append(atHead, schema.ValueAtHeadRow{
			TraceID:            traceIDs[i],
			MostRecentCommitID: "does not matter",
			Digest:             arbitraryBytes,
			OptionsID:          arbitraryBytes,
			GroupingID:         arbitraryBytes,
			Keys: paramtools.Params{
				types.CorpusField:   "corpus",
				"should_be_ignored": "true",
			},
			MatchesAnyIgnoreRule: schema.NBNull,
		})
	}
	for ; i < numTraces; i++ {
		traces = append(traces, schema.TraceRow{
			TraceID:    traceIDs[i],
			GroupingID: arbitraryBytes,
			Keys: paramtools.Params{
				types.CorpusField:   "corpus",
				"should_be_ignored": "false",
			},
			MatchesAnyIgnoreRule: schema.NBNull,
		})
		atHead = append(atHead, schema.ValueAtHeadRow{
			TraceID:            traceIDs[i],
			MostRecentCommitID: "does not matter",
			Digest:             arbitraryBytes,
			OptionsID:          arbitraryBytes,
			GroupingID:         arbitraryBytes,
			Keys: paramtools.Params{
				types.CorpusField:   "corpus",
				"should_be_ignored": "false",
			},
			MatchesAnyIgnoreRule: schema.NBNull,
		})
	}

	existingData := schema.Tables{
		Traces: traces,
		IgnoreRules: []schema.IgnoreRuleRow{{
			IgnoreRuleID: uuid.New(),
			CreatorEmail: "whomever",
			UpdatedEmail: "whomever",
			Expires:      time.Now(), // arbitrary
			Note:         "arbitrary",
			Query: paramtools.ReadOnlyParamSet{
				"should_be_ignored": []string{"true"},
			},
		}},
		ValuesAtHead: atHead,
	}
	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	// Verify things are null
	row := db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule IS NULL`)
	var count int
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, numTraces, count)
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule IS NULL`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, numTraces, count)

	require.NoError(t, UpdateIgnoredTraces(ctx, db))

	// Verify things are the right value
	row = db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule IS NULL`)
	count = -1
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 0, count)
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule IS NULL`)
	count = -1
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 0, count)

	row = db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule = TRUE`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, numTraces/3, count)
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule = TRUE`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, numTraces/3, count)
}

func TestUpdateIgnoredTraces_NullableRules_SetToCorrectValue(t *testing.T) {
	unittest.LargeTest(t)
	const whateverTS = "2020-12-01T00:00:00Z"
	b := databuilder.TablesBuilder{}
	b.CommitsWithData().
		Insert("whatever", "whatever", "whatever", whateverTS)

	b.SetDigests(map[rune]types.Digest{'A': dks.DigestA01Pos})
	b.SetGroupingKeys(types.CorpusField, types.PrimaryKeyField)
	// Based on problematic real-world data
	// trace_id 2ef4dd282af4c1daa44aafb9eeba6fff 402910fb035102d602cb8331d244af40
	b.AddTracesWithCommonKeys(paramtools.Params{
		"arch":             "x86_64",
		"compiler":         "Clang",
		"config":           "gldft",
		"cpu_or_gpu":       "GPU",
		"cpu_or_gpu_value": "GTX660",
		"model":            "ShuttleA",
		"name":             "texel_subset_nearest_mipmap_nearest_down",
		"os":               "Win10",
		"source_type":      "gm",
		"style":            "default",
	}).
		History("A", "A").
		Keys([]paramtools.Params{{"configuration": "Debug"}, {"configuration": "Release"}}).
		OptionsAll(paramtools.Params{"alpha_type": "Premul", "color_depth": "8888", "color_type": "RGBA_8888", "ext": "png", "gamut": "untagged", "transfer_fn": "untagged"}).
		IngestedFrom([]string{"whatever"}, []string{whateverTS})

	i := 0
	addIgnoreRule := func(ps paramtools.ParamSet) {
		b.AddIgnoreRule("whatever", "whatever", whateverTS, "whatever "+strconv.Itoa(i), ps)
		i++
	}

	// Was evaluating to NULL in prod for some traces.
	addIgnoreRule(paramtools.ParamSet{
		"cpu_or_gpu_value": []string{"GTX660", "GTX960"},
		// the above traces lack the extra_config key, and thus this had been returning null
		// (before the coalesce was added.
		"extra_config": []string{"Vulkan", "Vulkan_ProcDump"},
		"name":         []string{"texel_subset_nearest_mipmap_linear_down", "texel_subset_nearest_mipmap_nearest_down"},
		"os":           []string{"Win10"},
	})
	// emphasize the issue
	addIgnoreRule(paramtools.ParamSet{"key does not exist": []string{"whoops", "not", "here"}})
	// These rules were fine in production, but problematic when combined with above.
	addIgnoreRule(paramtools.ParamSet{
		"cpu_or_gpu":       []string{"GPU"},
		"cpu_or_gpu_value": []string{"Mali400MP2"},
		"name":             []string{"imagemakewithfilter", "imagemakewithfilter_crop", "imagemakewithfilter_crop_ref", "imagemakewithfilter_ref"}},
	)
	addIgnoreRule(paramtools.ParamSet{"config": []string{"ep3", "erec2020", "p3", "rec2020"}})

	existingData := b.Build()
	// Force these all to be null
	for i := range existingData.Traces {
		existingData.Traces[i].MatchesAnyIgnoreRule = schema.NBNull
	}
	for i := range existingData.ValuesAtHead {
		existingData.ValuesAtHead[i].MatchesAnyIgnoreRule = schema.NBNull
	}

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	// Verify things are null
	row := db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule IS NULL`)
	var count int
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 2, count)
	assert.Equal(t, 2, len(existingData.Traces))
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule IS NULL`)
	count = 0
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 2, count)
	assert.Equal(t, 2, len(existingData.ValuesAtHead))

	require.NoError(t, UpdateIgnoredTraces(ctx, db))
	// Verify things are not null
	row = db.QueryRow(ctx, `SELECT count(*) FROM Traces WHERE matches_any_ignore_rule IS NULL`)
	count = -1
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 0, count)
	row = db.QueryRow(ctx, `SELECT count(*) FROM ValuesAtHead WHERE matches_any_ignore_rule IS NULL`)
	count = -1
	require.NoError(t, row.Scan(&count))
	assert.Equal(t, 0, count)
}

func makeRandomTraceIDs(n int) []schema.TraceID {
	rv := make([]schema.TraceID, 0, n)
	for i := 0; i < n; i++ {
		t := make(schema.TraceID, md5.Size)
		_, err := rand.Read(t)
		if err != nil {
			panic(err)
		}
		rv = append(rv, t)
	}
	return rv
}
