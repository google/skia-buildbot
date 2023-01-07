// Package config contains the configuration for a running machineserver instance.
package config

// Source is configuration for the source of machine.Events.
type Source struct {
	// Project is the Google Cloud project that contains the pubsub topic.
	Project string `json:"project"`

	// The pubsub topic to listen to for machine events.
	Topic string `json:"topic"`
}

// DescriptionChangeSource is configuration for the source of pubsub events that
// arrive when the machine.Description of this test machine has changed.
type DescriptionChangeSource struct {
	// Project is the Google Cloud project that contains the pubsub topic.
	Project string `json:"project"`

	// The pubsub topic to listen to for events.
	Topic string `json:"topic"`
}

// Store is configuration for the datastore.
type Store struct {
	// Project is the Google Cloud project that contains the firestore database.
	Project string `json:"project"`

	// The instance of machine server (prod/test).
	Instance string `json:"instance"`
}

// InstanceConfig is the config for an instance of machineserver.
type InstanceConfig struct {
	Source                  Source                  `json:"source"`
	Store                   Store                   `json:"store"`
	DescriptionChangeSource DescriptionChangeSource `json:"desc_source"`

	// ConnectionString, if supplied, points to the CockroachDB database to use
	// for storage. If not supplied storage falls back to Firestore.
	ConnectionString string `json:"connection_string"`
}
