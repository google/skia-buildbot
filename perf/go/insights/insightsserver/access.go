// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package main

import (
	"net"
	"net/url"
	"strings"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/insights/localonly"
)

const (
	// uberproxyHostSuffix is the domain of the UberProxy PEN hostnames that serve Cloudtop ports,
	// e.g. https://<encoded address>.proxy.googlers.com. UberProxy sends them as the Host header.
	uberproxyHostSuffix = ".proxy.googlers.com"

	// readOnlyOption is the PostgreSQL setting that makes every transaction read-only. PGAdapter
	// maps it to Spanner read-only transactions, which also take no locks.
	readOnlyOption = "default_transaction_read_only=on"
)

// netConfig describes how the server is reached.
type netConfig struct {
	// PublicURL is the base URL shown to users and agents.
	PublicURL string

	// AllowedHosts, AllowedHostSuffixes and AllowedOrigins configure the Host and Origin checks.
	AllowedHosts        []string
	AllowedHostSuffixes []string
	AllowedOrigins      []string

	// RequireAuth requires the auth-proxy headers.
	RequireAuth bool
}

// isWildcardHost returns true for a listen host that means "all interfaces".
func isWildcardHost(host string) bool {
	return host == "" || host == "0.0.0.0" || host == "::"
}

// newNetConfig validates the listen address and the access mode and returns how the server is
// reached. hostname is the machine's hostname, used in the default public URL when listening on
// all interfaces.
//
// Access modes:
//   - loopback --listen (the default): only this machine can connect, no auth needed;
//   - non-loopback --listen: auth-proxy headers are required (fail closed);
//   - pen: a Cloudtop serves other Googlers through UberProxy PEN, which doesn't send auth-proxy
//     headers, so none are required.
func newNetConfig(listen, publicURL string, requireAuth, pen bool, hostname string) (*netConfig, error) {
	host, port, err := net.SplitHostPort(listen)
	if err != nil {
		return nil, skerr.Wrapf(err, "invalid --listen")
	}
	loopback := localonly.IsLoopbackHost(host)
	switch {
	case pen && loopback:
		return nil, skerr.Fmt("--uberproxy_pen needs a non-loopback --listen, e.g. :8090, because UberProxy can't reach loopback addresses")
	case pen && requireAuth:
		return nil, skerr.Fmt("--uberproxy_pen can't be combined with --require_auth, because UberProxy doesn't send auth-proxy headers")
	case !pen && !loopback && !requireAuth:
		sklog.Warningf("--listen=%s is not a loopback address, so --require_auth is turned on.", listen)
		requireAuth = true
	}

	base := strings.TrimRight(publicURL, "/")
	if base == "" {
		h := host
		if isWildcardHost(h) {
			h = hostname
		}
		base = "http://" + net.JoinHostPort(h, port)
	}
	pub, err := url.Parse(base)
	if err != nil || (pub.Scheme != "http" && pub.Scheme != "https") || pub.Host == "" || pub.Path != "" {
		return nil, skerr.Fmt("--public_url must be an http(s) URL without a path, e.g. https://perfinsights.example.com")
	}

	ret := &netConfig{
		PublicURL:      base,
		AllowedHosts:   []string{pub.Host},
		AllowedOrigins: []string{pub.Scheme + "://" + pub.Host},
		RequireAuth:    requireAuth,
	}
	if !isWildcardHost(host) {
		ret.AllowedHosts = append(ret.AllowedHosts, net.JoinHostPort(host, port))
	}
	if loopback || pen {
		// Requests from this machine, e.g. from the presenter's browser or agent.
		for _, h := range []string{"localhost", "127.0.0.1"} {
			ret.AllowedHosts = append(ret.AllowedHosts, net.JoinHostPort(h, port))
			ret.AllowedOrigins = append(ret.AllowedOrigins, "http://"+net.JoinHostPort(h, port))
		}
	}
	if pen {
		ret.AllowedHostSuffixes = []string{uberproxyHostSuffix}
	}
	return ret, nil
}

// readOnlyConnectionString returns connectionString with the option that makes every transaction
// read-only, so the database rejects writes even if a bug attempted one. perfinsights only reads.
func readOnlyConnectionString(connectionString string) (string, error) {
	u, err := url.Parse(connectionString)
	if err != nil || (u.Scheme != "postgresql" && u.Scheme != "postgres") {
		return "", skerr.Fmt("the connection string must be a postgresql:// URL")
	}
	q := u.Query()
	options := strings.TrimSpace(q.Get("options"))
	if strings.Contains(options, readOnlyOption) {
		return connectionString, nil
	}
	q.Set("options", strings.TrimSpace(options+" -c "+readOnlyOption))
	u.RawQuery = q.Encode()
	return u.String(), nil
}
