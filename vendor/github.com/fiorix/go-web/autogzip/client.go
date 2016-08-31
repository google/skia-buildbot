// Copyright 2013-2014 The go-web authors.  All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autogzip

import (
	"compress/gzip"
	"crypto/tls"
	"io/ioutil"
	"net/http"
)

// GetPage is an HTTP client that automatically decodes gzip when necessary.
func GetPage(url string) ([]byte, error) {
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	client := &http.Client{Transport: tr}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Accept-Encoding", "gzip, deflate")
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	var body []byte
	if resp.Header.Get("Content-Encoding") == "gzip" {
		var gz *gzip.Reader
		gz, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
		defer gz.Close()
		body, err = ioutil.ReadAll(gz)
	} else {
		body, err = ioutil.ReadAll(resp.Body)
	}
	if err != nil {
		return nil, err
	}
	return body, nil
}
