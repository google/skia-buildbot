// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
	"github.com/luci/luci-go/common/retry"
	"golang.org/x/net/context/ctxhttp"
)

type client struct {
	HTTP    *http.Client
	baseURL *url.URL
}

// call makes an HTTP call with exponential back-off on transient errors.
// if urlStr is relative, rebases it on c.baseURL.
func (c *client) call(ctx context.Context, method, urlStr string, body interface{}) (response []byte, err error) {
	if !c.baseURL.IsAbs() {
		panic("baseURL is not absolute")
	}

	var bodyBytes []byte
	if body != nil {
		var err error
		if bodyBytes, err = json.Marshal(body); err != nil {
			return nil, err
		}
	}

	u, err := url.Parse(urlStr)
	if err != nil {
		return nil, fmt.Errorf("invalid urlStr: %s", err)
	}
	u = c.baseURL.ResolveReference(u)
	logging.Infof(ctx, "%s %s", method, u)

	err = retry.Retry(
		ctx,
		retry.TransientOnly(retry.Default),
		func() error {
			req := &http.Request{Method: method, URL: u}
			if bodyBytes != nil {
				req.Body = ioutil.NopCloser(bytes.NewReader(bodyBytes))
			}
			res, err := ctxhttp.Do(ctx, c.HTTP, req)
			if err != nil {
				return errors.WrapTransient(err)
			}

			defer res.Body.Close()
			if res.StatusCode >= 500 {
				bodyBytes, _ := ioutil.ReadAll(res.Body)
				return errors.WrapTransient(fmt.Errorf(
					"status %s: %s", res.Status, bodyBytes))
			}

			response, err = ioutil.ReadAll(res.Body)
			return err
		},
		func(err error, wait time.Duration) {
			logging.WithError(err).Warningf(ctx, "API request failed transiently, will retry in %s", wait)
		},
	)
	return
}
