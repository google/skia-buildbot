package config

import (
	"time"
)

// tomlDuration is a simple struct wrapper to allow us to parse strings as durations
// from the incoming toml file (e.g,. RunEvery = "5m")
type TomlDuration struct {
	time.Duration
}

func (d *TomlDuration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

type Common struct {
	DoOAuth               bool   // Run through the OAuth 2.0 flow on startup, otherwise use a GCE service account.
	GitRepoDir            string // Directory location for the Skia repo.
	GraphiteServer        string // Where the Graphite metrics ingestion server is running.
	Local                 bool   // Running locally if true. As opposed to in production.
	TileDir               string // Path where tiles will be placed.
	OAuthCacheFile        string // Path to the file where to cache cache the oauth credentials.
	OAuthClientSecretFile string // Path to the file with the oauth client secret.
}
