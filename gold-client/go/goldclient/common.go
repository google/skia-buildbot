package goldclient

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/cenkalti/backoff"

	"go.skia.org/infra/go/fileutil"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// getWithRetries makes a GET request with retries to work around the rare unexpected EOF error.
// See https://crbug.com/skia/9108.
func getWithRetries(ctx context.Context, url string) ([]byte, error) {
	httpClient := extractHTTPClient(ctx)

	eb := backoff.NewExponentialBackOff()
	eb.InitialInterval = time.Second
	eb.MaxInterval = 10 * time.Second
	eb.MaxElapsedTime = 30 * time.Second

	var returnBytes []byte
	logAndReturn := func(err error) error {
		fmt.Printf("\t%s\n", err)
		return err
	}
	err := backoff.Retry(func() error {
		if err := ctx.Err(); err != nil {
			return backoff.Permanent(err)
		}
		resp, err := httpClient.Get(url)
		if err != nil {
			return logAndReturn(skerr.Wrapf(err, "GET %s", url))
		}

		if resp.StatusCode >= http.StatusBadRequest {
			return logAndReturn(skerr.Fmt("GET %s resulted in a %d: %s", url, resp.StatusCode, resp.Status))
		}
		defer func() {
			if err := resp.Body.Close(); err != nil {
				fmt.Printf("Warning while closing HTTP response for %s: %s", url, err)
			}
		}()
		returnBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return logAndReturn(skerr.Wrapf(err, "reading body from GET %s", url))
		}
		return nil
	}, eb)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return returnBytes, nil
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
