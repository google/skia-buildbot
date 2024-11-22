// This binary can be used to serve demo pages during development. It also serves as the test_on_env
// environment for Puppeteer tests.
package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

const (
	envPortFileBaseName = "port"
)

// A simple ResponseWriter wrapper that captures the response from the reverse proxy and prints its reponses for inspections.
type ResponseWriterWrapper struct {
	w          *http.ResponseWriter
	body       *bytes.Buffer
	statusCode *int
}

// NewResponseWriterWrapper static function creates a wrapper for the http.ResponseWriter
func NewResponseWriterWrapper(w http.ResponseWriter) ResponseWriterWrapper {
	var buf bytes.Buffer
	var statusCode int = 200
	return ResponseWriterWrapper{
		w:          &w,
		body:       &buf,
		statusCode: &statusCode,
	}
}

func (rww ResponseWriterWrapper) Write(buf []byte) (int, error) {
	rww.body.Write(buf)
	return (*rww.w).Write(buf)
}

// Header function overwrites the http.ResponseWriter Header() function
func (rww ResponseWriterWrapper) Header() http.Header {
	return (*rww.w).Header()

}

// WriteHeader function overwrites the http.ResponseWriter WriteHeader() function
func (rww ResponseWriterWrapper) WriteHeader(statusCode int) {
	(*rww.statusCode) = statusCode
	(*rww.w).WriteHeader(statusCode)
}

func (rww ResponseWriterWrapper) String() string {
	var buf bytes.Buffer

	buf.WriteString("Response:\n")
	buf.WriteString(fmt.Sprintf(" Status Code: %d\n", *(rww.statusCode)))

	buf.WriteString("Headers:\n")
	for k, v := range (*rww.w).Header() {
		buf.WriteString(fmt.Sprintf("%s: %v\n", k, v))
	}

	buf.WriteString("Body:\n")
	buf.WriteString(rww.body.String())
	return buf.String()
}

func proxyHandler(remote *url.URL, p *httputil.ReverseProxy) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		log.Println(r.URL, r.Method)
		log.Println(r.Header)

		// Only do plain test response so we can inspect the content easily.
		r.Header.Del("Accept-Encoding")
		bbytes, _ := io.ReadAll(r.Body)
		body := string(bbytes)
		log.Println(body)
		rw := NewResponseWriterWrapper(w)
		nb := io.NopCloser(strings.NewReader(body))
		r.Host = remote.Host
		r.Body = nb
		p.ServeHTTP(rw, r)
		log.Println(rw.String())
	}
}

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
		remote     *url.URL
		actualPort int
		err        error
	)

	// Provide an optional staging or prod endpoint that the the demo server reverse-proxied to.
	// e.g. https://perf.luci.app, or https://v8-perf.skia.org
	// Because there is no redirection or login, this needs to be publicly available.
	envRemoteEndpoint := os.Getenv("ENV_REMOTE_ENDPOINT")
	if envRemoteEndpoint != "" {
		if remote, err = url.Parse(envRemoteEndpoint); err != nil {
			panic(err)
		}
	}

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

	if remote != nil {
		fmt.Printf("Requests are reverse-proxied to: %s\n", remote.Hostname())
		proxy := httputil.NewSingleHostReverseProxy(remote)
		http.HandleFunc("/_/", proxyHandler(remote, proxy))
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Session cookie used by demo pages to determine whether they are being served by
		// webpack-dev-server or by an sk_demo_page_server Bazel rule.
		// TODO(lovisolo): Delete.
		http.SetCookie(w, &http.Cookie{
			Name:  "bazel",
			Value: "true",
		})

		if remote != nil {
			// This allows the frontend to skip mock.
			http.SetCookie(w, &http.Cookie{
				Name:  "proxy_endpoint",
				Value: remote.Hostname(),
			})
		}

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
		err = os.WriteFile(envPortFile, []byte(strconv.Itoa(actualPort)), 0644)
		if err != nil {
			panic(err)
		}

		// Then, we write the ready file. This signals the test_on_env runner script that we are ready.
		err = os.WriteFile(envReadyFile, []byte{}, 0644)
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
