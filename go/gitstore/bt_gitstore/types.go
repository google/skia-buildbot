package bt_gitstore

// BTConfig contains the BigTable configuration to define where the repo should be stored.
type BTConfig struct {
	ProjectID  string
	InstanceID string
	TableID    string
	Shards     int
}
