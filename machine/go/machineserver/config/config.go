// Package config contains the configuration for a running machineserver instance.
package config

// Source is configuration for the source of machine.Events.
type Source struct {
	// Project is the Google Cloud project that contains the pubsub topic.
	Project string `json:"project"`

	// The pubsub topic to listen to for machine events.
	Topic string `json:"topic"`
}

// Store is configuration for the datastore.
type Store struct {
	// Project is the Google Cloud project that contains the firestore database.
	Project string `json:"project"`

	// The instance of machine server (prod/test).
	Instance string `json:"instance"`
}

// Web is configuration for the web interface.
type Web struct {
	// AllowedUsers is the list of users, or domain names, that are allowed access to the app.
	AllowedUsers []string `json:"allowed_users"`

	// AllowedHosts is the list of hosts that are allowed to make requests to
	// the web UI via CORS requests.
	AllowedHosts []string `json:"allowed_hosts"`
}

// PowerCycle contains the config for power-cycling.
type PowerCycle struct {
	// Filename is the name of JSON5 file that contains information about the
	// machines that can be power-cycled.
	Filename string `json:"filename"`

	// ConnectOnStartup is true if the controller should connect to the
	// power-cycle devices on startup.
	ConnectOnStartup bool `json:"connect_on_startup"`
}

// InstanceConfig is the config for an instance of machineserver.
type InstanceConfig struct {
	Source     Source     `json:"source"`
	Store      Store      `json:"store"`
	Web        Web        `json:"web"`
	PowerCycle PowerCycle `json:"power_cycle"`
}
