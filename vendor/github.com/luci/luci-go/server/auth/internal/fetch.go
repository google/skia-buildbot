// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package internal

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"

	"golang.org/x/net/context"
	"golang.org/x/net/context/ctxhttp"
)

// ClientFactory knows how to produce http.Client that attach proper OAuth
// headers.
//
// If 'scopes' is empty, the factory should return a client that makes anonymous
// requests.
type ClientFactory func(c context.Context, scopes []string) (*http.Client, error)

var clientFactory ClientFactory

// RegisterClientFactory allows external module to provide implementation of
// the ClientFactory.
//
// This is needed to resolve module dependency cycle between server/auth and
// server/auth/internal.
//
// See init() in server/auth/client.go.
//
// If client factory is not set, Do(...) uses http.DefaultClient. This happens
// in unit tests for various auth/* subpackages.
func RegisterClientFactory(f ClientFactory) {
	if clientFactory != nil {
		panic("ClientFactory is already registered")
	}
	clientFactory = f
}

// Request represents one JSON REST API request.
type Request struct {
	Method  string            // HTTP method to use
	URL     string            // URL to access
	Scopes  []string          // OAuth2 scopes to authenticate with or anonymous call if empty
	Headers map[string]string // optional map with request headers
	Body    interface{}       // object to convert to JSON and send as body or []byte with the body
	Out     interface{}       // where to deserialize the response to
}

// Do performs an HTTP request with retries on transient errors.
//
// It can be used to make GET or DELETE requests (if Body is nil) or POST or PUT
// requests (if Body is not nil). In latter case the body will be serialized to
// JSON.
//
// Respects context's deadline and cancellation.
func (r *Request) Do(c context.Context) error {
	// Grab a client first. Use same client for all retry attempts.
	var client *http.Client
	if clientFactory != nil {
		var err error
		if client, err = clientFactory(c, r.Scopes); err != nil {
			return err
		}
	} else {
		client = http.DefaultClient
	}

	// Prepare a blob with the request body. Marshal it once, to avoid
	// remarshaling on retries.
	isJSON := false
	var bodyBlob []byte
	if blob, ok := r.Body.([]byte); ok {
		bodyBlob = blob
	} else if r.Body != nil {
		var err error
		if bodyBlob, err = json.Marshal(r.Body); err != nil {
			return err
		}
		isJSON = true
	}

	return fetchJSON(c, client, r.Out, func() (*http.Request, error) {
		req, err := http.NewRequest(r.Method, r.URL, bytes.NewReader(bodyBlob))
		if err != nil {
			return nil, err
		}
		for k, v := range r.Headers {
			req.Header.Set(k, v)
		}
		if isJSON {
			req.Header.Set("Content-Type", "application/json")
		}
		return req, nil
	})
}

// TODO(vadimsh): Add retries on HTTP 500.

// fetchJSON fetches JSON document by making a request using given client.
func fetchJSON(c context.Context, client *http.Client, val interface{}, f func() (*http.Request, error)) error {
	r, err := f()
	if err != nil {
		logging.Errorf(c, "auth: URL fetch failed - %s", err)
		return err
	}
	logging.Infof(c, "auth: %s %s", r.Method, r.URL)
	resp, err := ctxhttp.Do(c, client, r)
	if err != nil {
		logging.Errorf(c, "auth: URL fetch failed, can't connect - %s", err)
		return errors.WrapTransient(err)
	}
	defer func() {
		ioutil.ReadAll(resp.Body)
		resp.Body.Close()
	}()
	if resp.StatusCode >= 300 {
		body, _ := ioutil.ReadAll(resp.Body)
		// Opportunistically try to unmarshal the response. Works with JSON APIs.
		if val != nil {
			json.Unmarshal(body, val)
		}
		logging.Errorf(c, "auth: URL fetch failed - HTTP %d - %s", resp.StatusCode, string(body))
		err := fmt.Errorf("auth: HTTP code (%d) when fetching %s", resp.StatusCode, r.URL)
		if resp.StatusCode >= 500 {
			return errors.WrapTransient(err)
		}
		return err
	}
	if val != nil {
		if err = json.NewDecoder(resp.Body).Decode(val); err != nil {
			logging.Errorf(c, "auth: URL fetch failed, bad JSON - %s", err)
			return fmt.Errorf("auth: can't deserialize JSON at %q - %s", r.URL, err)
		}
	}
	return nil
}
