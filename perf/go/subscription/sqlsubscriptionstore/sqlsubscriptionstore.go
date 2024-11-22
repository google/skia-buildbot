// Package sqlsubscriptionstore implements subscription.Store using an SQL database.

package sqlsubscriptionstore

import (
	"context"

	"github.com/jackc/pgx/v4"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/sql/pool"
	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

// statement is an SQL statement identifier.
type statement int

const (
	// The identifiers for all the SQL statements used.
	insertSubscription statement = iota
	getSubscription
	getAllSubscriptions
)

// statements holds all the raw SQL statemens.
var statements = map[statement]string{
	getSubscription: `
		SELECT
			name,
			revision,
			bug_labels,
			hotlists,
			bug_component,
			bug_priority,
			bug_severity,
			bug_cc_emails,
			contact_email
		FROM
			Subscriptions
		WHERE
				name=$1
			AND
				revision=$2
		`,
	insertSubscription: `
		INSERT INTO
			Subscriptions (name, revision, bug_labels, hotlists, bug_component, bug_priority, bug_severity, bug_cc_emails, contact_email)
		VALUES
			($1, $2, $3, $4, $5, $6, $7, $8, $9)
	`,
	getAllSubscriptions: `
		Select
			*
		FROM
			Subscriptions
	`,
}

// SubscriptionStore implements the subscription.Store interface using an SQL
// database.
type SubscriptionStore struct {
	db pool.Pool
}

// New returns a new *SubscriptionStore.
func New(db pool.Pool) (*SubscriptionStore, error) {
	return &SubscriptionStore{
		db: db,
	}, nil
}

// GetSubscription implements the subscription.Store interface.
func (s *SubscriptionStore) GetSubscription(ctx context.Context, name string, revision string) (*pb.Subscription, error) {
	sub := &pb.Subscription{}
	if err := s.db.QueryRow(ctx, statements[getSubscription], name, revision).Scan(
		&sub.Name,
		&sub.Revision,
		&sub.BugLabels,
		&sub.Hotlists,
		&sub.BugComponent,
		&sub.BugPriority,
		&sub.BugSeverity,
		&sub.BugCcEmails,
		&sub.ContactEmail,
	); err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, skerr.Wrapf(err, "Failed to load subscription.")
	}
	return sub, nil
}

// InsertSubscriptions implements the subscription.Store interface.
func (s *SubscriptionStore) InsertSubscriptions(ctx context.Context, subs []*pb.Subscription) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return skerr.Wrap(err)
	}

	for _, sub := range subs {
		if _, err := tx.Exec(ctx, statements[insertSubscription], sub.Name, sub.Revision, sub.BugLabels, sub.Hotlists, sub.BugComponent, sub.BugPriority, sub.BugSeverity, sub.BugCcEmails, sub.ContactEmail); err != nil {
			if err := tx.Rollback(ctx); err != nil {
				sklog.Errorf("Failed on rollback: %s", err)
			}
			return skerr.Wrap(err)
		}
	}

	return tx.Commit(ctx)
}

// GetAllSubscriptions implements the subscription.Store interface.
// This function queries the db to fetch all the subscriptions.
func (s *SubscriptionStore) GetAllSubscriptions(ctx context.Context) ([]*pb.Subscription, error) {
	stmt := statements[getAllSubscriptions]
	rows, err := s.db.Query(ctx, stmt)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to load subscriptions.")
	}

	subscriptions := []*pb.Subscription{}
	for rows.Next() {
		sub := &pb.Subscription{}
		if err = rows.Scan(
			&sub.Name,
			&sub.Revision,
			&sub.BugLabels,
			&sub.Hotlists,
			&sub.BugComponent,
			&sub.BugPriority,
			&sub.BugSeverity,
			&sub.BugCcEmails,
			&sub.ContactEmail,
		); err != nil {
			return nil, skerr.Wrapf(err, "Failed to parse subscriptions.")
		} else {
			subscriptions = append(subscriptions, sub)
		}
	}

	return subscriptions, nil
}
