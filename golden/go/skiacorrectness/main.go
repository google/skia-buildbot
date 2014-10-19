package main

import (
	"flag"
	"net/http"

	"github.com/golang/glog"
)

// Command line flags.
var (
	port      = flag.String("port", ":9000", "HTTP service address (e.g., ':9000')")
	staticDir = flag.String("static", "./app", "Directory with static content to serve")

// TODO (stephana): Just ideas to be sorted out later
// tempDir   = flag.String("temp", "./.cache",
// "Directory to store temporary file and application cache")
)

func main() {
	flag.Parse()
	defer glog.Flush()

	http.Handle("/", http.FileServer(http.Dir(*staticDir)))

	glog.Infoln("Serving on http://127.0.0.1" + *port)
	glog.Fatal(http.ListenAndServe(*port, nil))
}
