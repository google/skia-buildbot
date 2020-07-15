package main

// This program emulates the meta data server in GCE and is intended to be used in the Skolo.
// It is derived from infra/skolo/go/metadata_server and designed to run in Kubernetes (but that
// is not strictly necessary).
// It also incorporates the functionality of skolo/go/get_oauth2_token, uniting the fetching
// and serving the oauth tokens in a single process.

import (
	"context"
	"flag"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/skmetadata"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "metadata_server"
)

type instanceMetadataMap map[string]map[string]string

func (m instanceMetadataMap) Get(instance, key string) (string, error) {
	rv, ok := m[instance][key]
	if !ok {
		return "", fmt.Errorf("Unknown instance or key.")
	}
	return rv, nil
}

type projectMetadataMap map[string]string

func (m projectMetadataMap) Get(key string) (string, error) {
	rv, ok := m[key]
	if !ok {
		return "", fmt.Errorf("Unknown key.")
	}
	return rv, nil
}

func main() {
	// Flags.
	var (
		port     = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
		promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
		confFile = flag.String("conf", "", "Configuration file that defines which tokens should be served.")
	)

	common.InitWithMust(
		APP_NAME,
		common.PrometheusOpt(promPort),
	)

	// TODO(borenet): Load these from a file?
	var pm projectMetadataMap = map[string]string{
		"mykey": "myvalue",
	}
	var im instanceMetadataMap = map[string]map[string]string{
		"inst": {
			"mykey2": "myvalue2",
		},
	}

	// Read the config file.
	configs, err := readConfigFile(*confFile)
	if err != nil {
		sklog.Fatalf("Error reading config file %q: %s", *confFile, err)
	}

	// svcAccountTokens maps [token_file][ServiceAccountToken] and ensures there is one instance
	// of ServiceAccountToken per token file. That instance is responsible for refreshing it.
	svcAccountTokens := make(map[string]*skmetadata.ServiceAccountToken, len(configs))

	// clientTokenMapping maps [ip | host_name | self] ServiceAccountToken.
	clientTokenMapping := make(map[string]*skmetadata.ServiceAccountToken, len(configs))
	for _, config := range configs {
		token, ok := svcAccountTokens[config.KeyFile]
		if !ok {
			token, err = skmetadata.NewServiceAccountToken(config.KeyFile, true)
			if err != nil {
				sklog.Fatalf("Error retrieving service account token: %s", err)
			}

			// Start the update loop as a background process.
			go token.UpdateLoop(context.Background())
			svcAccountTokens[config.KeyFile] = token
		}

		// Create one entry per client which is an IP address, a hostname or 'self'.
		for _, clientAddr := range config.Clients {
			clientTokenMapping[clientAddr] = token
		}
	}

	r := mux.NewRouter()
	skmetadata.SetupServer(r, pm, im, clientTokenMapping)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on http://localhost%s", *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
