package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skolo/go/service_accounts"
	"go.skia.org/infra/skolo/go/skmetadata"
)

const (
	// APP_NAME is the name of this app.
	APP_NAME = "metadata_server"
)

var (
	// Flags.
	port     = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	promPort = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")

	serviceAccountIpMapping = map[*service_accounts.ServiceAccount][]string{
		service_accounts.ChromeSwarming: {"*"},
		service_accounts.ChromiumSwarm:  {"*"},
		service_accounts.Jumphost:       {"self"},
		service_accounts.RpiMaster:      {"192.168.1.98", "192.168.1.99"},
	}
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
	common.InitWithMust(
		APP_NAME,
		common.PrometheusOpt(promPort),
		//common.CloudLoggingOpt(),
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

	hostname, err := os.Hostname()
	if err != nil {
		sklog.Fatal(err)
	}
	serviceAccounts, ok := service_accounts.JumphostServiceAccountMapping[hostname]
	if !ok {
		sklog.Fatalf("Hostname not in jumphost mapping: %s", hostname)
	}

	tokens := make(map[string]*skmetadata.ServiceAccountToken, len(serviceAccounts))
	tokenIpMapping := make(map[string]string, len(serviceAccounts))
	for _, acct := range serviceAccounts {
		ipAddrs, ok := serviceAccountIpMapping[acct]
		if !ok {
			sklog.Fatalf("Service account has no IP address mapping: %s", acct.Email)
		}
		tokenFile := fmt.Sprintf("/var/local/token_%s.json", acct.Nickname)

		for _, ipAddr := range ipAddrs {
			tokenIpMapping[ipAddr] = tokenFile
			if _, ok := tokens[tokenFile]; !ok {
				tok, err := skmetadata.NewServiceAccountToken(tokenFile, false)
				if err != nil {
					sklog.Fatal(err)
				}
				go tok.UpdateLoop(context.Background())
				tokens[tokenFile] = tok
			}
		}
	}
	tokenMapping := make(map[string]*skmetadata.ServiceAccountToken, len(tokenIpMapping))
	for ipAddr, tokenFile := range tokenIpMapping {
		tokenMapping[ipAddr] = tokens[tokenFile]
	}

	r := mux.NewRouter()
	skmetadata.SetupServer(r, pm, im, tokenMapping)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on http://localhost%s", *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
