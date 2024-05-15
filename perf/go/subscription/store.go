// Package subscription handles retrieving subscriptions.
package subscription

import (
	"context"

	pb "go.skia.org/infra/perf/go/subscription/proto/v1"
)

// Store is an interface for things that persists Subscription.
type Store interface {
	// GetSubscription retrieves parsed graph configs for the given id.
	GetSubscription(ctx context.Context, name string, revision string) (*pb.Subscription, error)

	// InsertSubscriptions inserts multiple subscription.
	InsertSubscriptions(ctx context.Context, subscription []*pb.Subscription) error

	// GetAllSubscriptions gets all the subscriptions unique by name
	GetAllSubscriptions(ctx context.Context) ([]*pb.Subscription, error)
}
