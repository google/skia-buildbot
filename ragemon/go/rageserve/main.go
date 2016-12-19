package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"go.skia.org/infra/ragemon/go/parser"
	"go.skia.org/infra/ragemon/go/store"
)

var (
	port     = flag.String("port", ":8000", "HTTP service address (e.g., ':8000')")
	storeDir = flag.String("store_dir", "/tmp/store", "The directory to store data in.")
)

var (
	st store.Store
)

func postHandler(w http.ResponseWriter, r *http.Request) {
	if r.Body == nil {
		httputils.ReportError(w, r, fmt.Errorf("Missing POST body."), "Missing POST body.")
		return
	}
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		httputils.ReportError(w, r, err, "Failed to read body.")
		return
	}
	util.Close(r.Body)
	meas, err := parser.PlainText(string(b))
	if err != nil {
		httputils.ReportError(w, r, err, "Invalid input")
		return
	}
	if err := st.Add(meas); err != nil {
		httputils.ReportError(w, r, err, "Failed to add points.")
	}
}

func main() {
	defer common.LogPanic()
	common.Init()
	var err error
	st, err = store.New(*storeDir)
	if err != nil {
		sklog.Fatalf("Failed to create Store: %s", err)
	}
	r := mux.NewRouter()
	r.HandleFunc("/new", postHandler)
	http.Handle("/", httputils.LoggingGzipRequestResponse(r))
	sklog.Infoln("Ready to serve.")
	sklog.Fatal(http.ListenAndServe(*port, nil))
}
