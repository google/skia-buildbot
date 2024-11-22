package schema

// FavoriteSchema represents the SQL schema of the Favorites table.
type FavoriteSchema struct {
	// Unique identifier of the favorite
	ID string `sql:"id UUID PRIMARY KEY DEFAULT gen_random_uuid()"`

	// The user to which this favorite belong. The user id
	// will be their email as returned by uber-proxy auth
	UserId string `sql:"user_id STRING NOT NULL"`

	// Name of the favorite
	Name string `sql:"name STRING"`

	// A URL attached to the favorite
	Url string `sql:"url STRING NOT NULL"`

	// A short description of what this favorite is about
	Description string `sql:"description STRING"`

	// Stored as a Unix timestamp.
	LastModified int `sql:"last_modified INT"`

	// Index used to query favorites based on user id
	byUserIdIndex struct{} `sql:"INDEX by_user_id (user_id)"`
}
