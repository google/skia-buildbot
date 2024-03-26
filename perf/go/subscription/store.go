// Package subscription handles retrieving subscriptions.
package subscription

import (
	"context"
)

// Subscription
type Subscription struct {
	Name         string   `json:"name"`
	Revision     string   `json:"revision"`
	BugLabels    []string `json:"bug_labels"`
	Hotlists     []string `json:"hotlists"`
	BugComponent string   `json:"bug_component"`
	BugCCEmails  []string `json:"bug_cc_emails"`
	ContactEmail string   `json:"contact_email"`
}

// Store is an interface for things that persists Subscription.
type Store interface {
	// GetSubscription retrieves parsed graph configs for the given id.
	GetSubscription(ctx context.Context, name string, revision string) (*Subscription, error)

	// InsertSubscriptions inserts multiple subscription.
	InsertSubscriptions(ctx context.Context, subscription []*Subscription) error
}
