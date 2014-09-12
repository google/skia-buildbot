package main

import (
	"flag"
	"net/http"
)

import (
	"github.com/golang/glog"
)

// flags
var (
	port      = flag.String("port", ":9000", "HTTP service address (e.g., ':9000')")
	staticDir = flag.String("static", "./app", "Directory with static content to serve")

// TODO (stephana): Just ideas to be sorted out later
// tempDir   = flag.String("temp", "./.cache",
// "Directory to store temporary file and application cache")
)

func main() {
	// parse the arguments
	flag.Parse()

	// // Static file handling
	http.Handle("/", http.FileServer(http.Dir(*staticDir)))

	// Wire up the resources

	// Start the server
	glog.Infoln("Serving on http://127.0.0.1" + *port)
	glog.Fatal(http.ListenAndServe(*port, nil))
}
