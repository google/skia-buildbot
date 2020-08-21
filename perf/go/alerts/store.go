package alerts

import "context"

// Store is the interface used to persist Alerts.
type Store interface {
	// Save can write a new, or update an existing, Config. New Configs will
	// have an ID of -1. On insert the ID of the Alert will be updated.
	Save(ctx context.Context, cfg *Alert) error

	// Delete removes the Alert with the given id.
	Delete(ctx context.Context, id int) error

	// List retrieves all the Alerts.
	//
	// If includeDeleted is true then deleted Alerts are also included in the
	// response.
	List(ctx context.Context, includeDeleted bool) ([]*Alert, error)
}
