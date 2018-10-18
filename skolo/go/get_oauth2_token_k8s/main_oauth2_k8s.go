package main

/*
	Obtain an OAuth2 token from a service account key. Expects to run as root in a cron job.
*/

import (
	"encoding/json"
	"flag"
	"io/ioutil"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/sklog"
	compute "google.golang.org/api/compute/v1"
)

func processServiceAccount(serviceAccountFile, tokenFile string) error {
	// serviceAccountFile := fmt.Sprintf("/etc/service_account_%s.json", nickname)
	// dest := fmt.Sprintf("/var/local/token_%s.json", nickname)

	src, err := auth.NewJWTServiceAccountTokenSource("#bogus", serviceAccountFile, compute.CloudPlatformScope, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		sklog.Fatal(err)
	}
	tok, err := src.Token()
	if err != nil {
		sklog.Fatal(err)
	}

	b, err := json.Marshal(tok)
	if err != nil {
		sklog.Fatal(err)
	}
	if err := ioutil.WriteFile(tokenFile, b, 0644); err != nil {
		sklog.Fatal(err)
	}

	sklog.Infof("Wrote new auth token: %s", tok.AccessToken[len(tok.AccessToken)-8:])
	return nil
}

type SvcAccountTokenFiles struct {
	KeyFile   string `json:"keyFile"`
	TokenFile string `json:"tokenFile"`
}

func readConfig(configFile string) ([]*SvcAccountTokenFiles, error) {
	return nil, nil
}

func main() {
	// Flags.
	var (
		configFile = flag.String("conf", "", "Config file")
	)

	// Initialize everything including the flags.
	common.Init()

	configs, err := readConfig(*configFile)
	if err != nil {
		sklog.Fatalf("Error reading config file %q: %s", *configFile, err)
	}

	sklog.Infof("Obtaining new auth token.")

	for _, conf := range configs {
		if err := processServiceAccount(conf.KeyFile, conf.TokenFile); err != nil {
			sklog.Fatal(err)
		}
	}
}
