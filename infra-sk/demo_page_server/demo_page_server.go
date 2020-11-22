// This binary can be used to serve demo pages during development. It also serves as the test_on_env
// environment for Puppeteer tests.
package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
)

const (
	envPortFileBaseName = "port"
)

func main() {
	var (
		assetsDir = flag.String("directory", "", "Path to directory to serve.")
		port      = flag.Int("port", 0, "TCP port (defaults to an unused port chosen by the OS).")
	)
	flag.Parse()

	// Are we running inside a test_on_env target (e.g. a Puppeteer test)? If so, we'll write out the
	// TCP port to $ENV_DIR/port so it can be picked up by the test, and we'll create $ENV_READY_FILE
	// to let the test_on_env runner know the server is ready to accept connections.
	//
	// Both $ENV_DIR and $ENV_READY_FILE are set by the test_on_env runner script.
	envDir := os.Getenv("ENV_DIR")
	envReadyFile := os.Getenv("ENV_READY_FILE")
	envPortFile := ""
	if envDir != "" {
		envPortFile = path.Join(envDir, envPortFileBaseName)
	}

	// Set up the HTTP server.
	assetsDirAbs, err := filepath.Abs(*assetsDir)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Serving directory %s\n", assetsDirAbs)
	http.Handle("/", http.FileServer(http.Dir(*assetsDir)))

	// If the port is unspecified (i.e. 0), listen on an unused port chosen by the OS.
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", *port))
	if err != nil {
		panic(err)
	}
	actualPort := listener.Addr().(*net.TCPAddr).Port // Chosen by the OS if unspecified.

	// Build the demo page URL.
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	url := fmt.Sprintf("http://%s:%d", hostname, actualPort)

	// We only write out these files when running inside a test_on_env target.
	if envReadyFile != "" && envPortFile != "" {
		// Write port file first.
		err = ioutil.WriteFile(envPortFile, []byte(strconv.Itoa(actualPort)), 0644)
		if err != nil {
			panic(err)
		}

		// Write ready file second. This signals that the environment is ready for the tests to execute.
		err = ioutil.WriteFile(envReadyFile, []byte{}, 0644)
		if err != nil {
			panic(err)
		}
	}

	// Start the server immediately after writing the ready file, if applicable.
	fmt.Printf("Serving demo page at: %s/\n", url)
	err = http.Serve(listener, nil) // Always returns a non-nil error.

	if err.Error() != "" {
		panic(err)
	}
}
