// Package subscription handles retrieving subscriptions.
package subscription

import (
	"context"

	"github.com/jackc/pgx/v4"

	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

// Store is an interface for things that persists Subscription.
type Store interface {
	// GetSubscription retrieves parsed graph configs for the given id.
	GetSubscription(ctx context.Context, name string, revision string) (*pb.Subscription, error)

	// InsertSubscriptions inserts multiple subscription.
	InsertSubscriptions(ctx context.Context, subscription []*pb.Subscription, tx pgx.Tx) error

	// GetAllSubscriptions gets all the subscriptions unique by name
	GetAllSubscriptions(ctx context.Context) ([]*pb.Subscription, error)

	// GetAllActiveSubscriptions gets all subscriptions that match the current version.
	GetAllActiveSubscriptions(ctx context.Context) ([]*pb.Subscription, error)
}
