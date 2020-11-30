// Example Go test for the test_on_env Bazel rule.
package main

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"path"
	"strconv"
	"testing"

	"go.skia.org/infra/go/testutils/unittest"
)

// mustReadPort reads environment server's TCP port from $ENV_DIR/port.
func mustReadPort() int {
	envDir := os.Getenv("ENV_DIR")
	if envDir == "" {
		panic(fmt.Sprintf("required environment variable ENV_DIR is unset"))
	}
	portFileBytes, err := ioutil.ReadFile(path.Join(envDir, envPortFileBaseName))
	if err != nil {
		panic(err)
	}
	port, err := strconv.Atoi(string(portFileBytes))
	if err != nil {
		panic(err)
	}
	return port
}

func TestOnEnv(t *testing.T) {
	unittest.SmallTest(t)

	// If this is running via "go test" instead of "bazel test" (e.g. Infra-Experimental-Small-Linux)
	// then the environment binary is not running, so there is nothing to test.
	if os.Getenv("TEST_TARGET") == "" {
		// Make it clear in the "go test -v" output that we are skipping the real tests.
		t.Run("running outside Bazel, nothing to test", func(t *testing.T) {})
		return
	}

	tests := []struct {
		path               string
		expectedStatusCode int
		expectedBody       string
	}{
		{
			path:               "/",
			expectedStatusCode: http.StatusNotFound,
		},
		{
			path:               "/echo",
			expectedStatusCode: http.StatusBadRequest,
		},
		{
			path:               "/echo?msg=hello%20world",
			expectedStatusCode: http.StatusOK,
			expectedBody:       "hello world\n",
		},
	}

	for _, tc := range tests {
		t.Run(tc.path, func(t *testing.T) {
			resp, err := http.Get(fmt.Sprintf("http://localhost:%d%s", mustReadPort(), tc.path))
			if err != nil {
				t.Fatalf("http.Get returned a non-nil error: %v", err)
			}
			if resp.StatusCode != tc.expectedStatusCode {
				t.Errorf("got status code: %d, want: %d", resp.StatusCode, tc.expectedStatusCode)
			}
			if tc.expectedBody != "" {
				bodyBytes, err := ioutil.ReadAll(resp.Body)
				if err != nil {
					t.Errorf("error reading HTTP response body: %v", err)
				}
				body := string(bodyBytes)
				if body != tc.expectedBody {
					t.Errorf("got HTTP response body: %q, want: %q", body, tc.expectedBody)
				}
			}
		})
	}
}
