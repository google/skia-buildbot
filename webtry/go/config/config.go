package config

type Webtry struct {
	UseChroot    bool
	Port         string
	ResourcePath string
	CachePath    string
	InoutPath    string
	UseMetadata  bool
}

var Fiddle Webtry
