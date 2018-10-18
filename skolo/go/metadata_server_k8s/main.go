package main

import (
	"flag"
	"fmt"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
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
		//common.CloudLoggingOpt(),
	)
	skiaversion.MustLogVersion()

	// TODO(borenet): Load these from a file?
	var pm projectMetadataMap = map[string]string{
		"mykey": "myvalue",
	}
	var im instanceMetadataMap = map[string]map[string]string{
		"inst": map[string]string{
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
	svcAccountTokens := make(map[string]*metadata.ServiceAccountToken, len(configs))

	// clientTokenMapping maps [ip | host_name | self] ServiceAccountToken.
	clientTokenMapping := make(map[string]metadata.TokenProvider, len(configs))
	for _, config := range configs {
		token, ok := svcAccountTokens[config.TokenFile]
		if !ok {
			token, err = metadata.NewServiceAccountToken(config.TokenFile)
			if err != nil {
				sklog.Fatalf("Error retrieving service account token: %s", err)
			}
			svcAccountTokens[config.TokenFile] = token
		}

		// Create one entry per client which is an IP address, a hostname or 'self'.
		for _, client := range config.Clients {
			clientTokenMapping[client] = token
		}
	}

	r := mux.NewRouter()
	metadata.SetupServer(r, pm, im, clientTokenMapping)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on http://localhost%s", *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
