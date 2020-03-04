package goldclient

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// HTTPClient makes it easier to mock out goldclient's dependencies on
// http.Client by representing a smaller interface.
type HTTPClient interface {
	Get(url string) (resp *http.Response, err error)
	Post(url, contentType string, body io.Reader) (resp *http.Response, err error)
}

const maxAttempts = 5

// GetGoldInstanceURL returns the URL for a given Gold instance id.
// This is usually a formulaic transform, but there are some special cases.
func GetGoldInstanceURL(instanceID string) string {
	if instanceID == instanceIDSkiaLegacy {
		return hostSkiaLegacy
	}
	if instanceID == instanceIDFuchsia {
		return hostFuchsiaCorp
	}
	return fmt.Sprintf(goldHostTemplate, instanceID)
}

// fetchWithRetries makes a generic HTTP request with retries to work around the rare unexpected
// EOF error. See https://crbug.com/skia/9108. The actual HTTP request is carried out in the
// fetch function provided as an argument. Parameters httpMethod and url are only used for logging.
//
// Note: http.Client retries certain kinds of request upon encountering network errors. See
// https://golang.org/pkg/net/http/#Transport for more. This function covers other errors.
//
// TODO(kjlubick): Add context.Context.
func fetchWithRetries(fetch func() (*http.Response, error), httpMethod, url string) ([]byte, error) {
	var lastErr error

	for attempts := 0; attempts < maxAttempts; attempts++ {
		if lastErr != nil {
			fmt.Printf("Retry attempt #%d after error: %s\n", attempts, lastErr)
			// reset the error
			lastErr = nil

			// Sleep to give the server time to recover, if needed.
			time.Sleep(time.Duration(500+rand.Int31n(1000)) * time.Millisecond)
		}

		// wrap in a function to make sure the defer resp.Body.Close() can
		// happen before we try again.
		b, err := func() ([]byte, error) {
			resp, err := fetch()
			if err != nil {
				return nil, skerr.Fmt("error on %s %s: %s", httpMethod, url, err)
			}

			if resp.StatusCode >= http.StatusBadRequest {
				return nil, skerr.Fmt("%s %s resulted in a %d: %s", httpMethod, url, resp.StatusCode, resp.Status)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					fmt.Printf("Warning while closing HTTP response for %s: %s", url, err)
				}
			}()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, skerr.Fmt("error reading body from %s: %s", url, err)
			}
			return b, nil
		}()
		if err != nil {
			lastErr = err
			continue
		}
		return b, nil
	}
	return nil, lastErr
}

// getWithRetries makes a GET request to the specified URL using the retry logic in
// fetchWithRetries.
//
// TODO(kjlubick): Add context.Context.
func getWithRetries(httpClient HTTPClient, url string) ([]byte, error) {
	fetch := func() (*http.Response, error) {
		return httpClient.Get(url)
	}
	return fetchWithRetries(fetch, "GET", url)
}

// postWithRetries makes a POST request to the specified URL using the retry logic
// in fetchWithRetries.
//
// TODO(kjlubick): Add context.Context.
func postWithRetries(httpClient HTTPClient, url, contentType string, body io.Reader) ([]byte, error) {
	fetch := func() (*http.Response, error) {
		return httpClient.Post(url, contentType, body)
	}
	return fetchWithRetries(fetch, "POST", url)
}

// loadJSONFile loads and parses the JSON in 'fileName'. If the file doesn't exist it returns
// (false, nil). If the first return value is true, 'data' contains the parse JSON data.
func loadJSONFile(fileName string, data interface{}) (bool, error) {
	if !fileutil.FileExists(fileName) {
		return false, nil
	}

	err := util.WithReadFile(fileName, func(r io.Reader) error {
		return json.NewDecoder(r).Decode(data)
	})
	if err != nil {
		return false, skerr.Wrapf(err, "reading/parsing JSON file: %s", fileName)
	}

	return true, nil
}

// saveJSONFile stores the given 'data' in a file with the given name
func saveJSONFile(fileName string, data interface{}) error {
	err := util.WithWriteFile(fileName, func(w io.Writer) error {
		return json.NewEncoder(w).Encode(data)
	})
	if err != nil {
		return skerr.Wrapf(err, "writing/serializing to JSON file %s", fileName)
	}
	return nil
}
