/*
	Central message passing app for Skia Infra.
*/

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
)

import (
	"github.com/gorilla/mux"
	"github.com/skia-dev/glog"
)

import (
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/skiaversion"
	"go.skia.org/infra/go/util"
)

// flags
var (
	graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
	host           = flag.String("host", "localhost", "HTTP service host")
	port           = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
	useMetadata    = flag.Bool("use_metadata", true, "Load sensitive values from metadata not from flags.")
	testing        = flag.Bool("testing", false, "Set to true for locally testing rules. No email will be sent.")
	workdir        = flag.String("workdir", ".", "Directory to use for scratch work.")
)

func mainHandler(w http.ResponseWriter, r *http.Request) {
	if _, err := w.Write([]byte(`
<html>
<body>
Hello World
</body>
</html>
`)); err != nil {
		glog.Error(err)
	}
}

func googleStorageChangeHandler(w http.ResponseWriter, r *http.Request) {
	defer util.Close(r.Body)
	var n struct {
		Kind           string            `json:"kind"`
		Id             string            `json:""`
		SelfLink       string            `json:"selfLink"`
		Name           string            `json:"name"`
		Bucket         string            `json:"bucket"`
		Generation     string            `json:"generation"`
		Metageneration string            `json:"metageneration"`
		ContentType    string            `json:"contentType"`
		Updated        string            `json:"updated"`
		TimeDeleted    string            `json:"timeDeleted"`
		StorageClass   string            `json:"storageClass"`
		Size           string            `json:"size"`
		Md5Hash        string            `json:"md5hash"`
		MediaLink      string            `json:"mediaLink"`
		Owner          map[string]string `json:"owner"`
		Crc32C         string            `json:"crc32c"`
		ETag           string            `json:"etag"`
	}
	if err := json.NewDecoder(r.Body).Decode(&n); err != nil {
		util.ReportError(w, r, err, fmt.Sprintf("Failed to read response body: %v", err))
	}
	glog.Infof("Google Storage notification from bucket \"%s\": %s", n.Bucket, n.Name)
}

func runServer(serverURL string) {
	r := mux.NewRouter()
	r.HandleFunc("/", mainHandler)
	r.HandleFunc("/googlestorage", googleStorageChangeHandler).Methods("POST")
	r.HandleFunc("/json/version", skiaversion.JsonHandler)
	http.Handle("/", util.LoggingGzipRequestResponse(r))
	glog.Infof("Ready to serve on %s", serverURL)
	glog.Fatal(http.ListenAndServe(*port, nil))
}

func main() {
	common.InitWithMetrics("grandcentral", graphiteServer)
	v, err := skiaversion.GetVersion()
	if err != nil {
		glog.Fatal(err)
	}
	glog.Infof("Version %s, built at %s", v.Commit, v.Date)

	if *testing {
		*useMetadata = false
	}
	serverURL := "https://" + *host
	if *testing {
		serverURL = "http://" + *host + *port
	}
	runServer(serverURL)
}
