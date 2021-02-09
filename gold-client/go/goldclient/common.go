package goldclient

import (
	"context"
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

const maxAttempts = 5

// getWithRetries makes a GET request with retries to work around the rare unexpected EOF error.
// See https://crbug.com/skia/9108.
//
// Note: http.Client retries certain kinds of request upon encountering network errors. See
// https://golang.org/pkg/net/http/#Transport for more. This function covers other errors.
func getWithRetries(ctx context.Context, url string) ([]byte, error) {
	var lastErr error
	httpClient := extractHTTPClient(ctx)

	for attempts := 0; attempts < maxAttempts; attempts++ {
		if err := ctx.Err(); err != nil {
			return nil, skerr.Wrapf(err, "context error")
		}
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
			resp, err := httpClient.Get(url)
			if err != nil {
				return nil, skerr.Fmt("error on GET %s: %s", url, err)
			}

			if resp.StatusCode >= http.StatusBadRequest {
				return nil, skerr.Fmt("GET %s resulted in a %d: %s", url, resp.StatusCode, resp.Status)
			}
			defer func() {
				if err := resp.Body.Close(); err != nil {
					fmt.Printf("Warning while closing HTTP response for %s: %s", url, err)
				}
			}()
			b, err := ioutil.ReadAll(resp.Body)
			if err != nil {
				return nil, skerr.Fmt("error reading body from GET %s: %s", url, err)
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

// post makes a POST request to the specified URL with the given body.
func post(ctx context.Context, url, contentType string, body io.Reader) ([]byte, error) {
	httpClient := extractHTTPClient(ctx)
	resp, err := httpClient.Post(url, contentType, body)
	if err != nil {
		return nil, skerr.Fmt("error on POST %s: %s", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= http.StatusBadRequest {
		return nil, skerr.Fmt("POST %s resulted in a %d: %s", url, resp.StatusCode, resp.Status)
	}

	bytes, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, skerr.Fmt("error reading body from POST %s: %s", url, err)
	}

	return bytes, nil
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
