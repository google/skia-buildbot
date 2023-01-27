// Package config contains the configuration for a running machineserver instance.
package config

// InstanceConfig is the config for an instance of machineserver.
type InstanceConfig struct {
	// ConnectionString, if supplied, points to the CockroachDB database to use
	// for storage. If not supplied storage falls back to Firestore.
	ConnectionString string `json:"connection_string"`
}
