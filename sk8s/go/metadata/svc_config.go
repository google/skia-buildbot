package main

import (
	"strings"

	"go.skia.org/infra/go/config"
	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/skerr"
)

// ServiceAccountConf is one entry in the configuration file that defines multiple service accounts
// in one file as an array. See readConfigFile function.
type ServiceAccountConf struct {
	Project string   `json:"project"`
	Email   string   `json:"email"`
	KeyFile string   `json:"keyFile"`
	Clients []string `json:"clients"`
}

func readConfigFile(confFile string) ([]*ServiceAccountConf, error) {
	if confFile == "" {
		return nil, skerr.Fmt("Must provide a config file.")
	}

	ret := []*ServiceAccountConf{}
	if err := config.ParseConfigFile(confFile, "", &ret); err != nil {
		return nil, skerr.Fmt("Error parsing configuration file: %s", err)
	}

	// Make sure the entries reference existing files and are consistent.
	for _, c := range ret {
		c.Email = strings.TrimSpace(c.Email)
		c.Project = strings.TrimSpace(c.Project)
		c.KeyFile = strings.TrimSpace(c.KeyFile)

		// Make sure there is no empty entry.
		if c.Project == "" || c.Email == "" || c.KeyFile == "" || len(c.Clients) == 0 {
			return nil, skerr.Fmt("No entry in config file %s can be empty.", confFile)
		}
		for _, oneClient := range c.Clients {
			if strings.TrimSpace(oneClient) == "" {
				return nil, skerr.Fmt("'clients' in file %s cannot contain empty strings", confFile)
			}
		}

		// Make sure the files exist. If they are internally invalid it will be caught when they are
		// parsed and used.
		if !fileutil.FileExists(c.KeyFile) {
			return nil, skerr.Fmt("File %q referenced in config file %q does not exist", c.KeyFile, confFile)
		}
	}

	return ret, nil
}
