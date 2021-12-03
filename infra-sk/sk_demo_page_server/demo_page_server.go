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
		port      = flag.Int("port", 0, "TCP port (ignored / chosen by the OS if $ENV_DIR is set; see the test_on_env Bazel rule).")
	)
	flag.Parse()

	// The ENV_DIR and ENV_READY_FILE environment variables are set by the test_on_env runner script.
	// We detect whether we are running inside a test_on_env target based on said variables.
	envDir := os.Getenv("ENV_DIR")
	envReadyFile := os.Getenv("ENV_READY_FILE")
	testOnEnv := envDir != "" && envReadyFile != ""

	// If we're running inside a test_on_env target then we ignore the --port flag and let the OS
	// choose a TCP port number for us. This prevents tests from failing with "address already in use"
	// errors due to Bazel running multiple test targets in parallel.
	if testOnEnv {
		*port = 0
	}

	var (
		listener   net.Listener
		actualPort int
		err        error
	)

	// If the port is unspecified (i.e. 0), an unused port will be chosen by the OS.
	if *port == 0 {
		listener, err = net.Listen("tcp", fmt.Sprintf(":%d", *port))
		if err != nil {
			panic(err)
		}
		actualPort = listener.Addr().(*net.TCPAddr).Port // Retrieve the port number chosen by the OS.
	} else {
		// Try opening the specified port, or repeatedly increase it by 1 until an unused port is found.
		// This allows developers to view multiple demo pages at the same time.
		actualPort = *port
		for {
			if actualPort > 65535 {
				panic("no unused TCP ports found")
			}
			listener, err = net.Listen("tcp", fmt.Sprintf(":%d", actualPort))
			if err != nil {
				actualPort++
			} else {
				break
			}
		}
	}

	// Set up the HTTP server.
	assetsDirAbs, err := filepath.Abs(*assetsDir)
	if err != nil {
		panic(err)
	}
	fmt.Printf("Serving directory %s\n", assetsDirAbs)
	fileServer := http.FileServer(http.Dir(*assetsDir))
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Session cookie used by demo pages to determine whether they are being served by
		// webpack-dev-server or by an sk_demo_page_server Bazel rule.
		http.SetCookie(w, &http.Cookie{
			Name:  "bazel",
			Value: "true",
		})
		fileServer.ServeHTTP(w, r)
	})

	// Build the demo page URL.
	hostname, err := os.Hostname()
	if err != nil {
		panic(err)
	}
	url := fmt.Sprintf("http://%s:%d", hostname, actualPort)

	// We need to signal the test_on_env runner script that we are ready to accept connections.
	if testOnEnv {
		// First, we write out the TCP port number. This will be read by the test target.
		envPortFile := path.Join(envDir, envPortFileBaseName)
		err = ioutil.WriteFile(envPortFile, []byte(strconv.Itoa(actualPort)), 0644)
		if err != nil {
			panic(err)
		}

		// Then, we write the ready file. This signals the test_on_env runner script that we are ready.
		err = ioutil.WriteFile(envReadyFile, []byte{}, 0644)
		if err != nil {
			panic(err)
		}
	}

	// Start the server immediately after writing the ready file.
	fmt.Printf("Serving demo page at: %s/\n", url)
	err = http.Serve(listener, nil) // Always returns a non-nil error.

	if err.Error() != "" {
		panic(err)
	}
}
