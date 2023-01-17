// Package config contains the configuration for a running machineserver instance.
package config

// Store is configuration for the datastore.
type Store struct {
	// Project is the Google Cloud project that contains the firestore database.
	Project string `json:"project"`

	// The instance of machine server (prod/test).
	Instance string `json:"instance"`
}

// InstanceConfig is the config for an instance of machineserver.
type InstanceConfig struct {
	Store Store `json:"store"`

	// ConnectionString, if supplied, points to the CockroachDB database to use
	// for storage. If not supplied storage falls back to Firestore.
	ConnectionString string `json:"connection_string"`
}
