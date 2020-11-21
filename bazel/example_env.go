// Example environment for the test_on_env Bazel rule.
package main

import (
	"fmt"
	"io/ioutil"
	"net"
	"net/http"
	"os"
	"path"
	"strconv"
	"time"
)

const (
	delaySeconds        = 3
	envPortFileBaseName = "port"
)

func main() {
	// Read in build paths to the ready and port files.
	envDir := os.Getenv("ENV_DIR")
	if envDir == "" {
		panic(fmt.Sprintf("required environment variable ENV_DIR is unset"))
	}
	envReadyFile := os.Getenv("ENV_READY_FILE")
	if envReadyFile == "" {
		panic(fmt.Sprintf("required environment variable ENV_READY_FILE is unset"))
	}
	envPortFile := path.Join(envDir, envPortFileBaseName)

	// We test the readiness check by simulating a delay.
	fmt.Printf("Simulating a %d-second delay before starting the HTTP server...\n", delaySeconds)
	time.Sleep(delaySeconds * time.Second)

	// Serve a plaintext echo endpoint.
	http.HandleFunc("/echo", func(w http.ResponseWriter, r *http.Request) {
		msg := r.URL.Query().Get("msg")
		if msg == "" {
			w.WriteHeader(http.StatusBadRequest)
			if _, err := fmt.Fprintln(w, `Error: Query parameter "msg" is empty or missing.`); err != nil {
				panic(err)
			}
		} else {
			if _, err := fmt.Fprintln(w, msg); err != nil {
				panic(err)
			}
		}
	})

	// Listen on an unused port chosen by the OS.
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		panic(err)
	}
	port := listener.Addr().(*net.TCPAddr).Port

	// Write port file first.
	err = ioutil.WriteFile(envPortFile, []byte(strconv.Itoa(port)), 0644)
	if err != nil {
		panic(err)
	}

	// Write ready file second. This signals that the environment is ready for the tests to execute.
	err = ioutil.WriteFile(envReadyFile, []byte{}, 0644)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Listening on port %d.\n", port)
	err = http.Serve(listener, nil) // Always returns a non-nil error.

	if err.Error() != "" {
		panic(err)
	}
}
