package main

import (
	"go.skia.org/infra/go/sklog"
)

type ServiceAccountConf struct {
	Project   string   `json:"project"`
	Email     string   `json:"email"`
	KeyFile   string   `json:"keyFile"`
	TokenFile string   `json:"tokenFile"`
	Clients   []string `json:"clients"`
}

func readConfigFile(confFile string) ([]*ServiceAccountConf, error) {
	if confFile == "" {
		return nil, sklog.FmtErrorf("Must provide a config file.")
	}

	return nil, nil
}
