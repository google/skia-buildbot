package sqlignorestore

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/sql/databuilder"
	dks "go.skia.org/infra/golden/go/sql/datakitchensink"
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

func TestCreate_AllIgnoreStatusesNull_TracesMatchingRuleUpdated(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := loadTestData()
	// Set everything to null
	for i := range existingData.Traces {
		existingData.Traces[i].MatchesAnyIgnoreRule = schema.NBNull
	}
	for i := range existingData.ValuesAtHead {
		existingData.ValuesAtHead[i].MatchesAnyIgnoreRule = schema.NBNull
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	store := New(db)
	require.NoError(t, store.Create(ctx, ignore.Rule{
		CreatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Query:     "model=Sailfish&os=Android",
		Note:      "skbug.com/1234",
	}))

	expectedStatuses := map[string]schema.NullableBool{
		"SailfishOne":   schema.NBTrue, // changed
		"SailfishTwo":   schema.NBTrue, // changed
		"SailfishThree": schema.NBTrue, // changed
		"BullheadOne":   schema.NBNull, // untouched
		"BullheadTwo":   schema.NBNull, // untouched
		"BullheadThree": schema.NBNull, // untouched
	}
	actualTraces := getTracesAndStatus(ctx, t, db, "model", "name")
	assert.Equal(t, expectedStatuses, actualTraces)

	actualValuesAtHead := getValuesAtHeadAndStatus(ctx, t, db, "model", "name")
	assert.Equal(t, expectedStatuses, actualValuesAtHead)
}

func TestCreate_RuleWithSomeMissingValues_TracesMatchingRuleUpdated(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := loadTestData()
	// Set everything to null
	for i := range existingData.Traces {
		existingData.Traces[i].MatchesAnyIgnoreRule = schema.NBNull
	}
	for i := range existingData.ValuesAtHead {
		existingData.ValuesAtHead[i].MatchesAnyIgnoreRule = schema.NBNull
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	store := New(db)
	require.NoError(t, store.Create(ctx, ignore.Rule{
		CreatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		// This rule has some models and names that don't exist. That shouldn't cause anything
		// to fail - only those traces that do match will be counted.
		Query: "model=Bullhead&model=Snorlax&model=Bellsprout&os=Android&name=Three&name=Two&name=Four",
		Note:  "skbug.com/1234",
	}))

	expectedStatuses := map[string]schema.NullableBool{
		"SailfishOne":   schema.NBNull, // untouched
		"SailfishTwo":   schema.NBNull, // untouched
		"SailfishThree": schema.NBNull, // untouched
		"BullheadOne":   schema.NBNull, // untouched
		"BullheadTwo":   schema.NBTrue, // changed
		"BullheadThree": schema.NBTrue, // changed
	}
	actualTraces := getTracesAndStatus(ctx, t, db, "model", "name")
	assert.Equal(t, expectedStatuses, actualTraces)

	actualValuesAtHead := getValuesAtHeadAndStatus(ctx, t, db, "model", "name")
	assert.Equal(t, expectedStatuses, actualValuesAtHead)
}

func TestCreate_RuleWithMissingKey_NothingUpdated(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := loadTestData()
	// Set everything to null
	for i := range existingData.Traces {
		existingData.Traces[i].MatchesAnyIgnoreRule = schema.NBNull
	}
	for i := range existingData.ValuesAtHead {
		existingData.ValuesAtHead[i].MatchesAnyIgnoreRule = schema.NBNull
	}
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	store := New(db)
	require.NoError(t, store.Create(ctx, ignore.Rule{
		CreatedBy: "me@example.com",
		Expires:   time.Date(2020, time.May, 11, 10, 9, 0, 0, time.UTC),
		Query:     "model=Snorlax&os=Android",
		Note:      "skbug.com/1234",
	}))

	expectedStatuses := map[string]schema.NullableBool{
		"SailfishOne":   schema.NBNull, // untouched
		"SailfishTwo":   schema.NBNull, // untouched
		"SailfishThree": schema.NBNull, // untouched
		"BullheadOne":   schema.NBNull, // untouched
		"BullheadTwo":   schema.NBNull, // untouched
		"BullheadThree": schema.NBNull, // untouched
	}
	actualTraces := getTracesAndStatus(ctx, t, db, "model", "name")
	assert.Equal(t, expectedStatuses, actualTraces)

	actualValuesAtHead := getValuesAtHeadAndStatus(ctx, t, db, "model", "name")
	assert.Equal(t, expectedStatuses, actualValuesAtHead)
}

// loadTestData creates 6 traces of varying ignore states (2 each of NULL, True, False) with
// a single ignore rule.
func loadTestData() schema.Tables {
	data := databuilder.TablesBuilder{}
	data.CommitsWithData().Insert("123", "whoever@example.com", "initial commit", "2021-01-11T16:00:00Z")
	data.SetDigests(map[rune]types.Digest{
		'a': dks.DigestA04Unt,
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
	return data.Build()
}

func getTracesAndStatus(ctx context.Context, t *testing.T, db *pgxpool.Pool, keys ...string) map[string]schema.NullableBool {
	require.NotEmpty(t, keys)
	rows := sqltest.GetAllRows(ctx, t, db, "Traces", &schema.TraceRow{}).([]schema.TraceRow)
	actualTraces := map[string]schema.NullableBool{}
	for _, r := range rows {
		combined := ""
		for _, key := range keys {
			combined += r.Keys[key]
		}
		actualTraces[combined] = r.MatchesAnyIgnoreRule
	}
	return actualTraces
}

func getValuesAtHeadAndStatus(ctx context.Context, t *testing.T, db *pgxpool.Pool, keys ...string) map[string]schema.NullableBool {
	require.NotEmpty(t, keys)
	rows := sqltest.GetAllRows(ctx, t, db, "ValuesAtHead", &schema.ValueAtHeadRow{}).([]schema.ValueAtHeadRow)
	actualValues := map[string]schema.NullableBool{}
	for _, r := range rows {
		combined := ""
		for _, key := range keys {
			combined += r.Keys[key]
		}
		actualValues[combined] = r.MatchesAnyIgnoreRule
	}
	return actualValues
}

func TestUpdate_ExistingRuleExpanded_AdditionalTracesIgnored(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	newExpires := time.Date(2022, time.January, 1, 1, 1, 1, 0, time.UTC)

	store := New(db)
	require.NoError(t, store.Update(ctx, ignore.Rule{
		ID:        idForRule(existingData.IgnoreRules, "Taimen"),
		CreatedBy: dks.UserTwo,
		UpdatedBy: dks.UserFour,
		Expires:   newExpires,
		// This rule previously ignored the square and circle test. Now it ignores all three tests.
		Query: "device=taimen&name=square&name=triangle&name=circle",
		Note:  "Should ignore all 3 tests now",
	}))

	actualTraces := getTracesAndStatus(ctx, t, db, "device", "name")
	assert.Equal(t, schema.NBTrue, actualTraces["taimencircle"])       // Still ignored
	assert.Equal(t, schema.NBTrue, actualTraces["taimensquare"])       // Still ignored
	assert.Equal(t, schema.NBTrue, actualTraces["taimentriangle"])     // Updated
	assert.Equal(t, schema.NBFalse, actualTraces["walleyeround rect"]) // not affected
	assert.Equal(t, schema.NBFalse, actualTraces["iPad6,3square"])     // not affected

	actualValuesAtHead := getValuesAtHeadAndStatus(ctx, t, db, "device", "name")
	assert.Equal(t, schema.NBTrue, actualValuesAtHead["taimencircle"])   // Still ignored
	assert.Equal(t, schema.NBTrue, actualValuesAtHead["taimensquare"])   // Still ignored
	assert.Equal(t, schema.NBTrue, actualValuesAtHead["taimentriangle"]) // Updated
	// walleye + round rect is not landed, so it is not in the table ValuesAtHead
	assert.Equal(t, schema.NBFalse, actualValuesAtHead["iPad6,3square"]) // not affected

	actualIgnoreRules := sqltest.GetAllRows(ctx, t, db, "IgnoreRules", &schema.IgnoreRuleRow{}).([]schema.IgnoreRuleRow)
	assert.ElementsMatch(t, []schema.IgnoreRuleRow{{
		IgnoreRuleID: uuid.MustParse(idForRule(existingData.IgnoreRules, "Taimen")),
		CreatorEmail: dks.UserTwo,
		UpdatedEmail: dks.UserFour,                    // Updated
		Expires:      newExpires,                      // Updated
		Note:         "Should ignore all 3 tests now", // Updated
		Query: paramtools.ReadOnlyParamSet{
			dks.DeviceKey:         []string{dks.TaimenDevice},
			types.PrimaryKeyField: []string{dks.CircleTest, dks.SquareTest, dks.TriangleTest},
		},
	}, {
		IgnoreRuleID: uuid.MustParse(idForRule(existingData.IgnoreRules, "expired")),
		CreatorEmail: dks.UserTwo,
		UpdatedEmail: dks.UserOne,
		Expires:      time.Date(2020, time.February, 14, 13, 12, 11, 0, time.UTC),
		Note:         "This rule has expired (and does not apply to anything)",
		Query: paramtools.ReadOnlyParamSet{
			dks.DeviceKey:     []string{"Nokia4"},
			types.CorpusField: []string{dks.CornersCorpus},
		},
	}}, actualIgnoreRules)
}

func TestUpdate_QueryNotChanged_IgnoreRuleUpdated(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	newExpires := time.Date(2022, time.January, 1, 1, 1, 1, 0, time.UTC)

	store := New(db)
	require.NoError(t, store.Update(ctx, ignore.Rule{
		ID:        idForRule(existingData.IgnoreRules, "Taimen"),
		CreatedBy: dks.UserTwo,
		UpdatedBy: dks.UserFour,
		Expires:   newExpires,
		Query:     "device=taimen&name=square&name=circle",
		Note:      "Note and expires was updated",
	}))

	actualTraces := getTracesAndStatus(ctx, t, db, "device", "name")
	assert.Equal(t, schema.NBTrue, actualTraces["taimencircle"])
	assert.Equal(t, schema.NBTrue, actualTraces["taimensquare"])
	assert.Equal(t, schema.NBFalse, actualTraces["taimentriangle"])
	assert.Equal(t, schema.NBFalse, actualTraces["walleyeround rect"])
	assert.Equal(t, schema.NBFalse, actualTraces["iPad6,3square"])

	actualValuesAtHead := getValuesAtHeadAndStatus(ctx, t, db, "device", "name")
	assert.Equal(t, schema.NBTrue, actualValuesAtHead["taimencircle"])
	assert.Equal(t, schema.NBTrue, actualValuesAtHead["taimensquare"])
	assert.Equal(t, schema.NBFalse, actualValuesAtHead["taimentriangle"])
	assert.Equal(t, schema.NBFalse, actualValuesAtHead["iPad6,3square"])

	actualIgnoreRules := sqltest.GetAllRows(ctx, t, db, "IgnoreRules", &schema.IgnoreRuleRow{}).([]schema.IgnoreRuleRow)
	assert.ElementsMatch(t, []schema.IgnoreRuleRow{{
		IgnoreRuleID: uuid.MustParse(idForRule(existingData.IgnoreRules, "Taimen")),
		CreatorEmail: dks.UserTwo,
		UpdatedEmail: dks.UserFour,                   // Updated
		Expires:      newExpires,                     // Updated
		Note:         "Note and expires was updated", // Updated
		Query: paramtools.ReadOnlyParamSet{
			dks.DeviceKey:         []string{dks.TaimenDevice},
			types.PrimaryKeyField: []string{dks.CircleTest, dks.SquareTest},
		},
	}, {
		IgnoreRuleID: uuid.MustParse(idForRule(existingData.IgnoreRules, "expired")),
		CreatorEmail: dks.UserTwo,
		UpdatedEmail: dks.UserOne,
		Expires:      time.Date(2020, time.February, 14, 13, 12, 11, 0, time.UTC),
		Note:         "This rule has expired (and does not apply to anything)",
		Query: paramtools.ReadOnlyParamSet{
			dks.DeviceKey:     []string{"Nokia4"},
			types.CorpusField: []string{dks.CornersCorpus},
		},
	}}, actualIgnoreRules)
}

func TestUpdate_ExistingRuleReduced_FewerTracesIgnored(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	newExpires := time.Date(2022, time.January, 1, 1, 1, 1, 0, time.UTC)

	store := New(db)
	require.NoError(t, store.Update(ctx, ignore.Rule{
		ID:        idForRule(existingData.IgnoreRules, "Taimen"),
		CreatedBy: dks.UserTwo,
		UpdatedBy: dks.UserFour,
		Expires:   newExpires,
		// This rule previously ignored the square and circle test. Now it ignores just the one.
		Query: "device=taimen&name=square",
		Note:  "Should ignore one test now",
	}))

	actualTraces := getTracesAndStatus(ctx, t, db, "device", "name")
	assert.Equal(t, schema.NBFalse, actualTraces["taimencircle"])      // updated
	assert.Equal(t, schema.NBTrue, actualTraces["taimensquare"])       // still ignored
	assert.Equal(t, schema.NBFalse, actualTraces["taimentriangle"])    // still not ignored
	assert.Equal(t, schema.NBFalse, actualTraces["walleyeround rect"]) // not affected
	assert.Equal(t, schema.NBFalse, actualTraces["iPad6,3square"])     // not affected

	actualValuesAtHead := getValuesAtHeadAndStatus(ctx, t, db, "device", "name")
	assert.Equal(t, schema.NBFalse, actualValuesAtHead["taimencircle"])   // updated
	assert.Equal(t, schema.NBTrue, actualValuesAtHead["taimensquare"])    // Still ignored
	assert.Equal(t, schema.NBFalse, actualValuesAtHead["taimentriangle"]) // still not ignored
	// walleye + round rect is not landed, so it is not in the table ValuesAtHead
	assert.Equal(t, schema.NBFalse, actualValuesAtHead["iPad6,3square"]) // not affected

	actualIgnoreRules := sqltest.GetAllRows(ctx, t, db, "IgnoreRules", &schema.IgnoreRuleRow{}).([]schema.IgnoreRuleRow)
	assert.ElementsMatch(t, []schema.IgnoreRuleRow{{
		IgnoreRuleID: uuid.MustParse(idForRule(existingData.IgnoreRules, "Taimen")),
		CreatorEmail: dks.UserTwo,
		UpdatedEmail: dks.UserFour,                 // Updated
		Expires:      newExpires,                   // Updated
		Note:         "Should ignore one test now", // Updated
		Query: paramtools.ReadOnlyParamSet{
			dks.DeviceKey:         []string{dks.TaimenDevice},
			types.PrimaryKeyField: []string{dks.SquareTest},
		},
	}, {
		IgnoreRuleID: uuid.MustParse(idForRule(existingData.IgnoreRules, "expired")),
		CreatorEmail: dks.UserTwo,
		UpdatedEmail: dks.UserOne,
		Expires:      time.Date(2020, time.February, 14, 13, 12, 11, 0, time.UTC),
		Note:         "This rule has expired (and does not apply to anything)",
		Query: paramtools.ReadOnlyParamSet{
			dks.DeviceKey:     []string{"Nokia4"},
			types.CorpusField: []string{dks.CornersCorpus},
		},
	}}, actualIgnoreRules)
}

func TestUpdate_ExistingRuleChanged_DifferentTracesIgnored(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))
	newExpires := time.Date(2022, time.January, 1, 1, 1, 1, 0, time.UTC)

	store := New(db)
	require.NoError(t, store.Update(ctx, ignore.Rule{
		ID:        idForRule(existingData.IgnoreRules, "Taimen"),
		CreatedBy: dks.UserTwo,
		UpdatedBy: dks.UserFour,
		Expires:   newExpires,
		// This rule previously ignored the taimen square and circle test. Now it ignores the
		// walleye device and two different tests.
		Query: "device=walleye&name=triangle&name=round rect",
		Note:  "Should ignore walleye now",
	}))

	actualTraces := getTracesAndStatus(ctx, t, db, "device", "name")
	assert.Equal(t, schema.NBFalse, actualTraces["taimencircle"])     // updated
	assert.Equal(t, schema.NBFalse, actualTraces["taimensquare"])     // updated
	assert.Equal(t, schema.NBFalse, actualTraces["taimentriangle"])   // still not ignored
	assert.Equal(t, schema.NBFalse, actualTraces["walleyecircle"])    // still not ignored
	assert.Equal(t, schema.NBFalse, actualTraces["walleyesquare"])    // updated
	assert.Equal(t, schema.NBTrue, actualTraces["walleyetriangle"])   // still not ignored
	assert.Equal(t, schema.NBTrue, actualTraces["walleyeround rect"]) // updated
	assert.Equal(t, schema.NBFalse, actualTraces["iPad6,3square"])    // not affected

	actualValuesAtHead := getValuesAtHeadAndStatus(ctx, t, db, "device", "name")
	assert.Equal(t, schema.NBFalse, actualValuesAtHead["taimencircle"])   // updated
	assert.Equal(t, schema.NBFalse, actualValuesAtHead["taimensquare"])   // updated
	assert.Equal(t, schema.NBFalse, actualValuesAtHead["taimentriangle"]) // still not ignored
	assert.Equal(t, schema.NBFalse, actualValuesAtHead["walleyecircle"])  // still not ignored
	// walleye + round rect is not landed, so it is not in the table ValuesAtHead
	assert.Equal(t, schema.NBTrue, actualValuesAtHead["walleyetriangle"]) // updated

	assert.Equal(t, schema.NBFalse, actualValuesAtHead["iPad6,3square"]) // not affected

	actualIgnoreRules := sqltest.GetAllRows(ctx, t, db, "IgnoreRules", &schema.IgnoreRuleRow{}).([]schema.IgnoreRuleRow)
	assert.ElementsMatch(t, []schema.IgnoreRuleRow{{
		IgnoreRuleID: uuid.MustParse(idForRule(existingData.IgnoreRules, "Taimen")),
		CreatorEmail: dks.UserTwo,
		UpdatedEmail: dks.UserFour,                // Updated
		Expires:      newExpires,                  // Updated
		Note:         "Should ignore walleye now", // Updated
		Query: paramtools.ReadOnlyParamSet{
			dks.DeviceKey:         []string{dks.WalleyeDevice},
			types.PrimaryKeyField: []string{dks.RoundRectTest, dks.TriangleTest},
		},
	}, {
		IgnoreRuleID: uuid.MustParse(idForRule(existingData.IgnoreRules, "expired")),
		CreatorEmail: dks.UserTwo,
		UpdatedEmail: dks.UserOne,
		Expires:      time.Date(2020, time.February, 14, 13, 12, 11, 0, time.UTC),
		Note:         "This rule has expired (and does not apply to anything)",
		Query: paramtools.ReadOnlyParamSet{
			dks.DeviceKey:     []string{"Nokia4"},
			types.CorpusField: []string{dks.CornersCorpus},
		},
	}}, actualIgnoreRules)
}

func idForRule(rules []schema.IgnoreRuleRow, s string) string {
	for _, r := range rules {
		if strings.Contains(r.Note, s) {
			return r.IgnoreRuleID.String()
		}
	}
	panic("Could not find rule with matching note")
}

func TestUpdate_InvalidID_ReturnsErrorAndNothingIsModified(t *testing.T) {
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

	require.Error(t, store.Update(ctx, ignore.Rule{
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

}

func TestList_Success(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTestsWithProductionSchema(ctx, t)
	existingData := dks.Build()
	require.NoError(t, sqltest.BulkInsertDataTables(ctx, db, existingData))

	findIDForRule := func(substr string) string {
		for _, r := range existingData.IgnoreRules {
			if strings.Contains(r.Note, substr) {
				return r.IgnoreRuleID.String()
			}
		}
		panic("could not find")
	}
	store := New(db)
	rules, err := store.List(ctx)
	require.NoError(t, err)
	assert.Equal(t, []ignore.Rule{{
		ID:        findIDForRule("expired"),
		CreatedBy: dks.UserTwo,
		UpdatedBy: dks.UserOne,
		Expires:   time.Date(2020, time.February, 14, 13, 12, 11, 0, time.UTC),
		Query:     "device=Nokia4&source_type=corners",
		Note:      "This rule has expired (and does not apply to anything)",
	}, {
		ID:        findIDForRule("Taimen"),
		CreatedBy: dks.UserTwo,
		UpdatedBy: dks.UserOne,
		Expires:   time.Date(2030, time.December, 30, 15, 16, 17, 0, time.UTC),
		Query:     "device=taimen&name=square&name=circle",
		Note:      "Taimen isn't drawing correctly enough yet",
	}}, rules)
}

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
