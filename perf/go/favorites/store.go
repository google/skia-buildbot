package favorites

import (
	"context"
)

// Favorite is a struct that represents a favorite.
type Favorite struct {
	ID           string
	UserId       string
	Name         string
	Url          string
	Description  string
	LastModified int64
}

type SaveRequest struct {
	UserId      string
	Name        string
	Url         string
	Description string
}

// Store is the interface used to persist Favorites.
type Store interface {
	// Get fetches a favorite with the given id from the db.
	Get(ctx context.Context, id string) (*Favorite, error)

	// Create inserts a new favorite into the db
	Create(ctx context.Context, req *SaveRequest) error

	// Update updates an existing favorite into the db based on id
	Update(ctx context.Context, req *SaveRequest, id string) error

	// Delete removes the Favorite with the given id.
	Delete(ctx context.Context, userId string, id string) error

	// List retrieves all the Favorites by user id (email).
	List(ctx context.Context, userId string) ([]*Favorite, error)

	// Liveness checks if the front end is still connected to
	// cockroachDB. This function does not have anything to do with
	// the store's function. The Favorites Store was arbitrarily
	// picked because of its lack of essential function.
	Liveness(ctx context.Context) error
}
