package favorites

import (
	"context"
)

// Favorite is a struct that represents a favorite.
type Favorite struct {
	ID           int64
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
	Get(ctx context.Context, id int64) (*Favorite, error)

	// Create inserts a new favorite into the db
	Create(ctx context.Context, req *SaveRequest) error

	// Update updates an existing favorite into the db based on id
	Update(ctx context.Context, req *SaveRequest, id int64) error

	// Delete removes the Favorite with the given id.
	Delete(ctx context.Context, id int64) error

	// List retrieves all the Favorites by user id (email).
	List(ctx context.Context, userId string) ([]*Favorite, error)
}
