// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package lhttp

import (
	"errors"
	"net"
	"net/url"
	"strings"
)

func hostRequiresSSL(host string) bool {
	host = strings.ToLower(host)
	return strings.HasSuffix(host, ".appspot.com")
}

// CheckURL ensures that the URL has a valid scheme, and that, if it is an
// appspot server, that it uses HTTPS.
//
// If no protocol is specified, the protocol defaults to https://.
func CheckURL(s string) (string, error) {
	u, err := url.Parse(s)
	if err != nil {
		return "", err
	}
	if u.Scheme == "" {
		s = "https://" + s
		u.Scheme = "https"
	}
	if u.Scheme != "https" && u.Scheme != "http" {
		return "", errors.New("Only http:// or https:// scheme is accepted.")
	}
	if u.Scheme != "https" && hostRequiresSSL(u.Host) {
		return "", errors.New("only https:// scheme is accepted for appspot hosts, it can be omitted")
	}
	if _, err = url.Parse(s); err != nil {
		return "", err
	}
	return s, nil
}

// IsLocalHost returns true if hostport is local.
func IsLocalHost(hostport string) bool {
	host, _, err := net.SplitHostPort(hostport)
	if err != nil {
		return false
	}
	switch {
	case host == "localhost", host == "":
	case net.ParseIP(host).IsLoopback():

	default:
		return false
	}
	return true
}
