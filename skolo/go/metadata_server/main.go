package main

import (
	"context"
	"flag"
	"fmt"
	"net/http"
	"strings"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metadata"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/sklog"
)

var (
	// APP_NAME is the name of this app.
	APP_NAME = "metadata_server"

	// Flags.
	port       = flag.String("port", ":8000", "HTTP service port for the web server (e.g., ':8000')")
	promPort   = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
	tokenFiles = common.NewMultiStringFlag("token_file", nil, "Mapping of IP address to token file, eg. \"127.0.0.1:/path/to/token.json\"")
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

	if *tokenFiles == nil {
		sklog.Fatal("At least one --token_file must be specified.")
	}
	tokenMapping := make(map[string]*metadata.ServiceAccountToken, len(*tokenFiles))
	for _, f := range *tokenFiles {
		split := strings.Split(f, ":")
		if len(split) != 2 {
			sklog.Fatalf("Invalid value for --token_file: %s", f)
		}
		ipAddr := split[0]
		tokenFile := split[1]
		tok, err := metadata.NewServiceAccountToken(tokenFile)
		if err != nil {
			sklog.Fatal(err)
		}
		go tok.UpdateLoop(context.Background())
		tokenMapping[ipAddr] = tok
	}

	r := mux.NewRouter()
	metadata.SetupServer(r, pm, im, tokenMapping)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infof("Ready to serve on http://localhost%s", *port)
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
