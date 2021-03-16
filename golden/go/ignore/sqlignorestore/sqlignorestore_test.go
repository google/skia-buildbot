package sqlignorestore

import (
	"context"
	"testing"
	"time"

	"github.com/jackc/pgx/v4/pgxpool"

	"github.com/jackc/pgtype"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/sql"
	"go.skia.org/infra/golden/go/sql/databuilder"
	"go.skia.org/infra/golden/go/sql/datakitchensink"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
	"go.skia.org/infra/golden/go/types"
)

func TestCreate_RulesAppearInSQLTableAndCanBeListed(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db)

	require.NoError(t, store.Create(ctx, ignore.Rule{
		CreatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Query:     "model=NvidiaShield2015",
		Note:      "skbug.com/1234",
	}))
	require.NoError(t, store.Create(ctx, ignore.Rule{
		CreatedBy: "otheruser@example.com",
		Expires:   time.Date(2018, time.January, 10, 10, 10, 0, 0, time.UTC),
		Query:     "model=Pixel1&os=foo&model=Pixel2",
		Note:      "skbug.com/54678",
	}))

	// It's good to query the database directly for at least one test, so we can verify List()
	// is returning the proper data.
	actualRows := sqltest.GetAllRows(ctx, t, db, "IgnoreRules", &schema.IgnoreRuleRow{}).([]schema.IgnoreRuleRow)
	require.Len(t, actualRows, 2)
	firstID := actualRows[0].IgnoreRuleID
	secondID := actualRows[1].IgnoreRuleID
	assert.Equal(t, []schema.IgnoreRuleRow{{
		IgnoreRuleID: firstID,
		CreatorEmail: "otheruser@example.com",
		UpdatedEmail: "otheruser@example.com",
		Expires:      time.Date(2018, time.January, 10, 10, 10, 0, 0, time.UTC),
		Note:         "skbug.com/54678",
		Query: paramtools.ReadOnlyParamSet{
			"model": []string{"Pixel1", "Pixel2"},
			"os":    []string{"foo"},
		},
	}, {
		IgnoreRuleID: secondID,
		CreatorEmail: "me@example.com",
		UpdatedEmail: "me@example.com",
		Expires:      time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Note:         "skbug.com/1234",
		Query:        paramtools.ReadOnlyParamSet{"model": []string{"NvidiaShield2015"}},
	}}, actualRows)

	rules, err := store.List(ctx)
	require.NoError(t, err)

	assert.Equal(t, []ignore.Rule{{
		ID:        firstID.String(),
		CreatedBy: "otheruser@example.com",
		UpdatedBy: "otheruser@example.com",
		Expires:   time.Date(2018, time.January, 10, 10, 10, 0, 0, time.UTC),
		Query:     "model=Pixel1&model=Pixel2&os=foo",
		Note:      "skbug.com/54678",
	}, {
		ID:        secondID.String(),
		CreatedBy: "me@example.com",
		UpdatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Query:     "model=NvidiaShield2015",
		Note:      "skbug.com/1234",
	}}, rules)
}
func TestCreate_InvalidQuery_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	store := New(nil)
	require.Error(t, store.Create(context.Background(), ignore.Rule{
		CreatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Query:     "%NOT A VALID QUERY",
		Note:      "skbug.com/1234",
	}))
}

func TestCreate_AllTracesUpdated(t *testing.T) {
	t.Skip("Broken")
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	loadTestData(t, ctx, db)

	store := New(db)
	require.NoError(t, store.Create(ctx, ignore.Rule{
		CreatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Query:     "model=Sailfish&os=Android",
		Note:      "skbug.com/1234",
	}))

	rows, err := db.Query(ctx, `SELECT trace_id, corpus, grouping_id, keys, matches_any_ignore_rule FROM Traces`)
	require.NoError(t, err)
	defer rows.Close()
	var actualTraces []schema.TraceRow
	for rows.Next() {
		var r schema.TraceRow
		var matches pgtype.Bool
		require.NoError(t, rows.Scan(&r.TraceID, &r.Corpus, &r.GroupingID, &r.Keys, &matches))
		r.MatchesAnyIgnoreRule = convertToNullableBool(matches)
		actualTraces = append(actualTraces, r)
	}
	assert.ElementsMatch(t, []schema.TraceRow{
		traceRow(paramtools.Params{"os": "Android", "model": "Sailfish", "name": "One"}, schema.NBTrue), // changed
		traceRow(paramtools.Params{"os": "Android", "model": "Sailfish", "name": "Two"}, schema.NBTrue),
		traceRow(paramtools.Params{"os": "Android", "model": "Sailfish", "name": "Three"}, schema.NBTrue), // changed
		traceRow(paramtools.Params{"os": "Android", "model": "Bullhead", "name": "One"}, schema.NBFalse),  // changed
		traceRow(paramtools.Params{"os": "Android", "model": "Bullhead", "name": "Two"}, schema.NBTrue),   // still ignored
		traceRow(paramtools.Params{"os": "Android", "model": "Bullhead", "name": "Three"}, schema.NBFalse),
	}, actualTraces)

	counts, err := db.Query(ctx, `SELECT keys->>'model', keys->>'name', matches_any_ignore_rule FROM ValuesAtHead`)
	require.NoError(t, err)
	defer counts.Close()
	actualValuesAtHead := map[string]schema.NullableBool{}
	for counts.Next() {
		var model string
		var name string
		var matches pgtype.Bool
		require.NoError(t, counts.Scan(&model, &name, &matches))
		actualValuesAtHead[model+name] = convertToNullableBool(matches)
	}
	assert.Equal(t, map[string]schema.NullableBool{
		"SailfishOne":   schema.NBTrue, // changed
		"SailfishTwo":   schema.NBTrue,
		"SailfishThree": schema.NBTrue,  // changed
		"BullheadOne":   schema.NBFalse, // changed
		"BullheadTwo":   schema.NBTrue,  // still ignored
		"BullheadThree": schema.NBFalse,
	}, actualValuesAtHead)
}

// loadTestData creates 6 traces of varying ignore states (2 each of NULL, True, False) with
// a single ignore rule.
func loadTestData(t *testing.T, ctx context.Context, db *pgxpool.Pool) {
	data := databuilder.TablesBuilder{}
	data.CommitsWithData().Insert("123", "whoever@example.com", "initial commit", "2021-01-11T16:00:00Z")
	data.SetDigests(map[rune]types.Digest{
		'a': datakitchensink.DigestA04Unt,
	})
	data.SetGroupingKeys(types.CorpusField, types.PrimaryKeyField)
	data.AddTracesWithCommonKeys(paramtools.Params{types.CorpusField: "gm", "os": "Android"}).
		History("a", "a", "a", "a", "a", "a").Keys([]paramtools.Params{
		{"model": "Sailfish", types.PrimaryKeyField: "One"},
		{"model": "Sailfish", types.PrimaryKeyField: "Two"},
		{"model": "Sailfish", types.PrimaryKeyField: "Three"},
		{"model": "Bullhead", types.PrimaryKeyField: "One"},
		{"model": "Bullhead", types.PrimaryKeyField: "Two"},
		{"model": "Bullhead", types.PrimaryKeyField: "Three"},
	}).OptionsAll(paramtools.Params{"ext": "png"}).
		IngestedFrom([]string{"file"}, []string{"2021-01-11T16:05:00Z"})
	data.AddIgnoreRule("me@example.com", "me@example.com", "2021-01-11T17:00:00Z", "ignore test 2",
		paramtools.ParamSet{types.PrimaryKeyField: []string{"Two"}})
	b := data.Build()
	b.Traces[0].MatchesAnyIgnoreRule = schema.NBNull // pretend test 1 is null
	b.Traces[3].MatchesAnyIgnoreRule = schema.NBNull // pretend test 1 is null
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, b))
}

func convertToNullableBool(b pgtype.Bool) schema.NullableBool {
	if b.Status != pgtype.Present {
		return schema.NBNull
	}
	if b.Bool {
		return schema.NBTrue
	}
	return schema.NBFalse
}

func traceRow(params paramtools.Params, ignoreState schema.NullableBool) schema.TraceRow {
	params[types.CorpusField] = "gm"
	_, traceID := sql.SerializeMap(params)
	grouping := paramtools.Params{
		types.CorpusField: "gm", types.PrimaryKeyField: params[types.PrimaryKeyField],
	}
	_, groupingID := sql.SerializeMap(grouping)
	return schema.TraceRow{
		TraceID:              traceID,
		Corpus:               "gm",
		GroupingID:           groupingID,
		Keys:                 params,
		MatchesAnyIgnoreRule: ignoreState,
	}
}

func TestUpdate_ExistingRule_RuleIsModified(t *testing.T) {
	t.Skip("Broken")
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db)

	require.NoError(t, store.Create(ctx, ignore.Rule{
		CreatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Query:     "model=NvidiaShield2015",
		Note:      "skbug.com/1234",
	}))

	rules, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	recordID := rules[0].ID

	require.NoError(t, store.Update(ctx, ignore.Rule{
		ID:        recordID,
		UpdatedBy: "updator@example.com",
		Expires:   time.Date(2020, time.August, 3, 3, 3, 3, 0, time.UTC),
		Query:     "model=NvidiaShield2015&model=Pixel3",
		Note:      "See skbug.com/1234 for more",
	}))

	rules, err = store.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, ignore.Rule{
		ID:        recordID,
		CreatedBy: "me@example.com",
		UpdatedBy: "updator@example.com",
		Expires:   time.Date(2020, time.August, 3, 3, 3, 3, 0, time.UTC),
		Query:     "model=NvidiaShield2015&model=Pixel3",
		Note:      "See skbug.com/1234 for more",
	}, rules[0])
}

func TestUpdate_InvalidID_NothingIsModified(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db)

	require.NoError(t, store.Create(ctx, ignore.Rule{
		CreatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Query:     "model=NvidiaShield2015",
		Note:      "skbug.com/1234",
	}))

	rules, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	recordID := rules[0].ID

	require.NoError(t, store.Update(ctx, ignore.Rule{
		ID:        "00000000-1111-2222-3333-444444444444",
		UpdatedBy: "updator@example.com",
		Expires:   time.Date(2020, time.August, 3, 3, 3, 3, 0, time.UTC),
		Query:     "model=NvidiaShield2015&model=Pixel3",
		Note:      "See skbug.com/1234 for more",
	}))

	rules, err = store.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)
	assert.Equal(t, ignore.Rule{
		ID:        recordID,
		CreatedBy: "me@example.com",
		UpdatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Query:     "model=NvidiaShield2015",
		Note:      "skbug.com/1234",
	}, rules[0])
}

func TestUpdated_AllTracesUpdated(t *testing.T) {
	t.Skip("Broken")
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	loadTestData(t, ctx, db)

	store := New(db)
	rules, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)

	r := rules[0]
	r.Query = "model=Sailfish&os=Android"
	require.NoError(t, store.Update(ctx, r))

	rows, err := db.Query(ctx, `SELECT trace_id, corpus, grouping_id, keys, matches_any_ignore_rule FROM Traces`)
	require.NoError(t, err)
	defer rows.Close()
	var actualTraces []schema.TraceRow
	for rows.Next() {
		var r schema.TraceRow
		var matches pgtype.Bool
		require.NoError(t, rows.Scan(&r.TraceID, &r.Corpus, &r.GroupingID, &r.Keys, &matches))
		r.MatchesAnyIgnoreRule = convertToNullableBool(matches)
		actualTraces = append(actualTraces, r)
	}
	assert.ElementsMatch(t, []schema.TraceRow{
		traceRow(paramtools.Params{"os": "Android", "model": "Sailfish", "name": "One"}, schema.NBTrue), // changed
		traceRow(paramtools.Params{"os": "Android", "model": "Sailfish", "name": "Two"}, schema.NBTrue),
		traceRow(paramtools.Params{"os": "Android", "model": "Sailfish", "name": "Three"}, schema.NBTrue), // changed
		traceRow(paramtools.Params{"os": "Android", "model": "Bullhead", "name": "One"}, schema.NBFalse),  // changed
		traceRow(paramtools.Params{"os": "Android", "model": "Bullhead", "name": "Two"}, schema.NBFalse),  // changed
		traceRow(paramtools.Params{"os": "Android", "model": "Bullhead", "name": "Three"}, schema.NBFalse),
	}, actualTraces)

	counts, err := db.Query(ctx, `SELECT keys->>'model', keys->>'name', matches_any_ignore_rule FROM ValuesAtHead`)
	require.NoError(t, err)
	defer counts.Close()
	actualValuesAtHead := map[string]schema.NullableBool{}
	for counts.Next() {
		var model string
		var name string
		var matches pgtype.Bool
		require.NoError(t, counts.Scan(&model, &name, &matches))
		actualValuesAtHead[model+name] = convertToNullableBool(matches)
	}
	assert.Equal(t, map[string]schema.NullableBool{
		"SailfishOne":   schema.NBTrue, // changed
		"SailfishTwo":   schema.NBTrue,
		"SailfishThree": schema.NBTrue,  // changed
		"BullheadOne":   schema.NBFalse, // changed
		"BullheadTwo":   schema.NBFalse, // changed
		"BullheadThree": schema.NBFalse,
	}, actualValuesAtHead)
}

func TestUpdate_InvalidQuery_ReturnsError(t *testing.T) {
	unittest.SmallTest(t)

	store := New(nil)
	require.Error(t, store.Update(context.Background(), ignore.Rule{
		CreatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Query:     "%NOT A VALID QUERY",
		Note:      "skbug.com/1234",
	}))
}

func TestDelete_ExistingRule_RuleIsDeleted(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db)

	require.NoError(t, store.Create(ctx, ignore.Rule{
		CreatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Query:     "model=NvidiaShield2015",
		Note:      "skbug.com/1234",
	}))
	require.NoError(t, store.Create(ctx, ignore.Rule{
		CreatedBy: "otheruser@example.com",
		Expires:   time.Date(2018, time.January, 10, 10, 10, 0, 0, time.UTC),
		Query:     "model=Pixel1&os=foo&model=Pixel2",
		Note:      "skbug.com/54678",
	}))

	rules, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 2)
	firstID := rules[0].ID
	secondID := rules[1].ID

	require.NoError(t, store.Delete(ctx, secondID))

	rules, err = store.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)

	assert.Equal(t, ignore.Rule{
		ID:        firstID,
		CreatedBy: "otheruser@example.com",
		UpdatedBy: "otheruser@example.com",
		Expires:   time.Date(2018, time.January, 10, 10, 10, 0, 0, time.UTC),
		Query:     "model=Pixel1&model=Pixel2&os=foo",
		Note:      "skbug.com/54678",
	}, rules[0])
}

func TestDelete_MissingID_NothingIsDeleted(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	store := New(db)

	require.NoError(t, store.Create(ctx, ignore.Rule{
		CreatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Query:     "model=NvidiaShield2015",
		Note:      "skbug.com/1234",
	}))

	rules, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)

	// It is extremely unlikely this is the actual record ID.
	require.NoError(t, store.Delete(ctx, "00000000-1111-2222-3333-444444444444"))

	rules, err = store.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)
}

func TestDelete_NoRulesRemain_NothingIsIgnored(t *testing.T) {
	t.Skip("Broken")
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	loadTestData(t, ctx, db)

	store := New(db)
	rules, err := store.List(ctx)
	require.NoError(t, err)
	require.Len(t, rules, 1)

	require.NoError(t, store.Delete(ctx, rules[0].ID))

	rows, err := db.Query(ctx, `SELECT trace_id, corpus, grouping_id, keys, matches_any_ignore_rule FROM Traces`)
	require.NoError(t, err)
	defer rows.Close()
	var actualTraces []schema.TraceRow
	for rows.Next() {
		var r schema.TraceRow
		var matches pgtype.Bool
		require.NoError(t, rows.Scan(&r.TraceID, &r.Corpus, &r.GroupingID, &r.Keys, &matches))
		r.MatchesAnyIgnoreRule = convertToNullableBool(matches)
		actualTraces = append(actualTraces, r)
	}
	assert.ElementsMatch(t, []schema.TraceRow{
		traceRow(paramtools.Params{"os": "Android", "model": "Sailfish", "name": "One"}, schema.NBFalse), // changed
		traceRow(paramtools.Params{"os": "Android", "model": "Sailfish", "name": "Two"}, schema.NBFalse), // changed
		traceRow(paramtools.Params{"os": "Android", "model": "Sailfish", "name": "Three"}, schema.NBFalse),
		traceRow(paramtools.Params{"os": "Android", "model": "Bullhead", "name": "One"}, schema.NBFalse), // changed
		traceRow(paramtools.Params{"os": "Android", "model": "Bullhead", "name": "Two"}, schema.NBFalse),
		traceRow(paramtools.Params{"os": "Android", "model": "Bullhead", "name": "Three"}, schema.NBFalse),
	}, actualTraces)

	counts, err := db.Query(ctx, `SELECT keys->>'model', keys->>'name', matches_any_ignore_rule FROM ValuesAtHead`)
	require.NoError(t, err)
	defer counts.Close()
	for counts.Next() {
		var model string
		var name string
		var matches pgtype.Bool
		require.NoError(t, counts.Scan(&model, &name, &matches))
		actual := convertToNullableBool(matches)
		assert.Equal(t, schema.NBFalse, actual, "Value at head %s %s", model, name)
	}
}

func TestConvertIgnoreRules_Success(t *testing.T) {
	unittest.SmallTest(t)

	condition, args := convertIgnoreRules(nil)
	assert.Equal(t, "false", condition)
	assert.Empty(t, args)

	condition, args = convertIgnoreRules([]paramtools.ParamSet{
		{
			"key1": []string{"alpha"},
		},
	})
	assert.Equal(t, `((keys ->> $1::STRING IN ($2)))`, condition)
	assert.Equal(t, []interface{}{"key1", "alpha"}, args)

	condition, args = convertIgnoreRules([]paramtools.ParamSet{
		{
			"key1": []string{"alpha", "beta"},
			"key2": []string{"gamma"},
		},
		{
			"key3": []string{"delta", "epsilon", "zeta"},
		},
	})
	const expectedCondition = `((keys ->> $1::STRING IN ($2, $3) AND keys ->> $4::STRING IN ($5))
OR (keys ->> $6::STRING IN ($7, $8, $9)))`
	assert.Equal(t, expectedCondition, condition)
	assert.Equal(t, []interface{}{"key1", "alpha", "beta", "key2", "gamma", "key3", "delta", "epsilon", "zeta"}, args)
}
