package sqlsubscriptionstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/subscription"
)

func setUp(t *testing.T) (subscription.Store, pool.Pool) {
	db := sqltest.NewCockroachDBForTests(t, "subscriptionstore")
	store, err := New(db)
	require.NoError(t, err)

	return store, db
}

// Insert two valid subscriptions and ensure they're queryable.
func TestInsert_ValidSubscriptions(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)

	s := []*subscription.Subscription{
		{
			Name:         "Test Subscription 1",
			Revision:     "abcd",
			BugLabels:    []string{"A", "B"},
			Hotlists:     []string{"C", "D"},
			BugComponent: "Component1>Subcomponent1",
			BugCCEmails: []string{
				"abcd@efg.com",
				"1234@567.com",
			},
			ContactEmail: "test@owner.com",
		},
		{
			Name:         "Test Subscription 2",
			Revision:     "abcd",
			BugLabels:    []string{"1", "2"},
			Hotlists:     []string{"3", "4"},
			BugComponent: "Component2>Subcomponent2",
			BugCCEmails: []string{
				"abcd@efg.com",
				"1234@567.com",
			},
			ContactEmail: "test@owner.com",
		},
	}

	err := store.InsertSubscriptions(ctx, s)
	require.NoError(t, err)

	actual := getSubscriptionsFromDb(t, ctx, db)
	assert.ElementsMatch(t, actual, s)
}

// Test inserting two subscriptions with same primary key. The transaction
// should fail on second insert and no subscriptions should live in the DB
// due to transaction rollback.
func TestInsert_DuplicateSubscriptionKeys(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)

	s := []*subscription.Subscription{
		{
			Name:         "Test Subscription 1",
			Revision:     "abcd",
			BugLabels:    []string{"A", "B"},
			Hotlists:     []string{"C", "D"},
			BugComponent: "Component1>Subcomponent1",
			BugCCEmails: []string{
				"abcd@efg.com",
				"1234@567.com",
			},
			ContactEmail: "test@owner.com",
		},
		{
			Name:         "Test Subscription 1",
			Revision:     "abcd",
			BugLabels:    []string{"1", "2"},
			Hotlists:     []string{"3", "4"},
			BugComponent: "Component2>Subcomponent2",
			BugCCEmails: []string{
				"abcd@efg.com",
				"1234@567.com",
			},
			ContactEmail: "test@owner.com",
		},
	}

	err := store.InsertSubscriptions(ctx, s)
	require.Error(t, err)

	actual := getSubscriptionsFromDb(t, ctx, db)
	assert.Empty(t, actual)
}

func TestInsert_EmptyList(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)

	s := []*subscription.Subscription{}

	err := store.InsertSubscriptions(ctx, s)
	require.NoError(t, err)

	actual := getSubscriptionsFromDb(t, ctx, db)
	assert.Empty(t, actual)
}

func TestGet_ValidSubscription(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)

	s := &subscription.Subscription{
		Name:         "Test Subscription 1",
		Revision:     "abcd",
		BugLabels:    []string{"A", "B"},
		Hotlists:     []string{"C", "D"},
		BugComponent: "Component1>Subcomponent1",
		BugCCEmails: []string{
			"abcd@efg.com",
			"1234@567.com",
		},
		ContactEmail: "test@owner.com",
	}
	insertSubscriptionToDb(t, ctx, db, s)
	actual, err := store.GetSubscription(ctx, "Test Subscription 1", "abcd")
	require.NoError(t, err)

	assert.Equal(t, actual, s)
}

// Test that fails when retrieving an unknown subscription.
func TestGet_NonExistent(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	_, err := store.GetSubscription(ctx, "Fake Subscription", "abcd")
	require.Error(t, err)
}

func insertSubscriptionToDb(t *testing.T, ctx context.Context, db pool.Pool, subscription *subscription.Subscription) {
	const query = `INSERT INTO Subscriptions
        (name, revision, bug_labels, hotlists, bug_component, bug_cc_emails, contact_email)
        VALUES ($1,$2,$3,$4,$5,$6,$7)`
	if _, err := db.Exec(ctx, query, subscription.Name, subscription.Revision, subscription.BugLabels, subscription.Hotlists, subscription.BugComponent, subscription.BugCCEmails, subscription.ContactEmail); err != nil {
		require.NoError(t, err)
	}
}

func getSubscriptionsFromDb(t *testing.T, ctx context.Context, db pool.Pool) []*subscription.Subscription {
	actual := []*subscription.Subscription{}
	rows, _ := db.Query(ctx, "SELECT name, revision, bug_labels, hotlists, bug_component, bug_cc_emails, contact_email FROM Subscriptions")
	for rows.Next() {
		subscriptionInDb := new(subscription.Subscription)
		if err := rows.Scan(&subscriptionInDb.Name, &subscriptionInDb.Revision, &subscriptionInDb.BugLabels, &subscriptionInDb.Hotlists, &subscriptionInDb.BugComponent, &subscriptionInDb.BugCCEmails, &subscriptionInDb.ContactEmail); err != nil {
			require.NoError(t, err)
		}
		actual = append(actual, subscriptionInDb)
	}
	return actual
}
