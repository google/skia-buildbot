package sqlignorestore

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/testutils/unittest"
	"go.skia.org/infra/golden/go/ignore"
	"go.skia.org/infra/golden/go/sql/schema"
	"go.skia.org/infra/golden/go/sql/sqltest"
)

func TestCreate_RulesAppearInSQLTableAndCanBeListed(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)
	sqltest.CreateProductionSchema(ctx, t, db)
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
	rows, err := db.Query(ctx, `SELECT * FROM IgnoreRules ORDER BY expires ASC`)
	require.NoError(t, err)
	defer rows.Close()
	var actualRows []schema.IgnoreRuleRow
	for rows.Next() {
		var r schema.IgnoreRuleRow
		assert.NoError(t, rows.Scan(&r.IgnoreRuleID, &r.CreatorEmail, &r.UpdatedEmail, &r.Expires, &r.Note, &r.Query))
		r.Expires = r.Expires.UTC()
		actualRows = append(actualRows, r)
	}
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

func TestUpdate_ExistingRule_RuleIsModified(t *testing.T) {
	unittest.LargeTest(t)

	ctx := context.Background()
	db := sqltest.NewCockroachDBForTests(ctx, t)
	sqltest.CreateProductionSchema(ctx, t, db)
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
	db := sqltest.NewCockroachDBForTests(ctx, t)
	sqltest.CreateProductionSchema(ctx, t, db)
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
	db := sqltest.NewCockroachDBForTests(ctx, t)
	sqltest.CreateProductionSchema(ctx, t, db)
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
	db := sqltest.NewCockroachDBForTests(ctx, t)
	sqltest.CreateProductionSchema(ctx, t, db)
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
