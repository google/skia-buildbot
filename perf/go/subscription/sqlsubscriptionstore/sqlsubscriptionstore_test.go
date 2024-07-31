package sqlsubscriptionstore

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/sql/pool"
	"go.skia.org/infra/perf/go/sql/sqltest"
	"go.skia.org/infra/perf/go/subscription"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
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

	s := []*pb.Subscription{
		{
			Name:         "Test Subscription 1",
			Revision:     "abcd",
			BugLabels:    []string{"A", "B"},
			Hotlists:     []string{"C", "D"},
			BugComponent: "Component1>Subcomponent1",
			BugPriority:  1,
			BugSeverity:  2,
			BugCcEmails: []string{
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
			BugPriority:  1,
			BugSeverity:  2,
			BugCcEmails: []string{
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

	s := []*pb.Subscription{
		{
			Name:         "Test Subscription 1",
			Revision:     "abcd",
			BugLabels:    []string{"A", "B"},
			Hotlists:     []string{"C", "D"},
			BugComponent: "Component1>Subcomponent1",
			BugPriority:  1,
			BugSeverity:  2,
			BugCcEmails: []string{
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
			BugPriority:  1,
			BugSeverity:  2,
			BugCcEmails: []string{
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

	s := []*pb.Subscription{}

	err := store.InsertSubscriptions(ctx, s)
	require.NoError(t, err)

	actual := getSubscriptionsFromDb(t, ctx, db)
	assert.Empty(t, actual)
}

func TestGet_ValidSubscription(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)

	s := &pb.Subscription{
		Name:         "Test Subscription 1",
		Revision:     "abcd",
		BugLabels:    []string{"A", "B"},
		Hotlists:     []string{"C", "D"},
		BugComponent: "Component1>Subcomponent1",
		BugPriority:  1,
		BugSeverity:  2,
		BugCcEmails: []string{
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

func TestGet_AllSubscriptionsUniqByName(t *testing.T) {
	ctx := context.Background()
	store, db := setUp(t)

	s := &pb.Subscription{
		Name:         "Test Subscription 1",
		Revision:     "abcd",
		BugLabels:    []string{"A", "B"},
		Hotlists:     []string{"C", "D"},
		BugComponent: "Component1>Subcomponent1",
		BugPriority:  1,
		BugSeverity:  2,
		BugCcEmails: []string{
			"abcd@efg.com",
			"1234@567.com",
		},
		ContactEmail: "test@owner.com",
	}

	s1 := &pb.Subscription{
		Name:         "Test Subscription 2",
		Revision:     "bcde",
		BugLabels:    []string{"A", "B"},
		Hotlists:     []string{"C", "D"},
		BugComponent: "Component1>Subcomponent1",
		BugPriority:  1,
		BugSeverity:  2,
		BugCcEmails: []string{
			"abcd@efg.com",
			"1234@567.com",
		},
		ContactEmail: "test@owner.com",
	}

	insertSubscriptionToDb(t, ctx, db, s)
	insertSubscriptionToDb(t, ctx, db, s1)

	actual, err := store.GetAllSubscriptions(ctx)
	require.NoError(t, err)

	expected := []*pb.Subscription{s, s1}
	assert.Equal(t, actual, expected)
}

// Test that checks nil is returned when retrieving a non-existent subscription.
func TestGet_NonExistent(t *testing.T) {
	ctx := context.Background()
	store, _ := setUp(t)

	sub, err := store.GetSubscription(ctx, "Fake Subscription", "abcd")
	require.NoError(t, err)
	assert.Nil(t, sub)
}

func insertSubscriptionToDb(t *testing.T, ctx context.Context, db pool.Pool, subscription *pb.Subscription) {
	const query = `INSERT INTO Subscriptions
        (name, revision, bug_labels, hotlists, bug_component, bug_priority, bug_severity, bug_cc_emails, contact_email)
        VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9)`
	if _, err := db.Exec(ctx, query, subscription.Name, subscription.Revision, subscription.BugLabels, subscription.Hotlists, subscription.BugComponent, subscription.BugPriority, subscription.BugSeverity, subscription.BugCcEmails, subscription.ContactEmail); err != nil {
		require.NoError(t, err)
	}
}

func getSubscriptionsFromDb(t *testing.T, ctx context.Context, db pool.Pool) []*pb.Subscription {
	actual := []*pb.Subscription{}
	rows, _ := db.Query(ctx, "SELECT name, revision, bug_labels, hotlists,  bug_component, bug_priority, bug_severity, bug_cc_emails, contact_email FROM Subscriptions")
	for rows.Next() {
		subscriptionInDb := &pb.Subscription{}
		if err := rows.Scan(&subscriptionInDb.Name, &subscriptionInDb.Revision, &subscriptionInDb.BugLabels, &subscriptionInDb.Hotlists, &subscriptionInDb.BugComponent, &subscriptionInDb.BugPriority, &subscriptionInDb.BugSeverity, &subscriptionInDb.BugCcEmails, &subscriptionInDb.ContactEmail); err != nil {
			require.NoError(t, err)
		}
		actual = append(actual, subscriptionInDb)
	}
	return actual
}
