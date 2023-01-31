// Package config contains the configuration for a running machineserver instance.
package config

// Pool describes a single pool of machines.
type Pool struct {
	// Name of the pool as it will appear in Dimensions at the machine.DimPool key.
	Name string `json:"name"`

	// Regex is a regular expression that matches a machine id if that machine
	// is in this pool.
	Regex string `json:"regex"`
}

// InstanceConfig is the config for an instance of machineserver.
type InstanceConfig struct {
	// ConnectionString, if supplied, points to the CockroachDB database to use
	// for storage. If not supplied storage falls back to Firestore.
	ConnectionString string `json:"connection_string"`

	// Pools is a list of Pools. They are evaluated in the order they appear in
	// the config file.
	Pools []Pool `json:"pools"`
}
