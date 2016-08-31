// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/maruel/subcommands"
	"golang.org/x/net/context"

	"net/url"

	"github.com/luci/luci-go/common/auth"
	"github.com/luci/luci-go/common/lhttp"
	"github.com/luci/luci-go/common/logging"
)

type baseCommandRun struct {
	subcommands.CommandRunBase
	serviceAccountJSONPath string
	host                   string
}

func (r *baseCommandRun) SetDefaultFlags() {
	r.Flags.StringVar(
		&r.serviceAccountJSONPath,
		"service-account-json",
		"",
		"path to service account json file.")
	r.Flags.StringVar(
		&r.host,
		"host",
		"cr-buildbucket.appspot.com",
		"host for the buildbucket service instance.")
}

func (r *baseCommandRun) createClient(ctx context.Context) (*client, error) {
	if r.host == "" {
		return nil, fmt.Errorf("a host for the buildbucket service must be provided")
	}
	if strings.ContainsRune(r.host, '/') {
		return nil, fmt.Errorf("invalid host %q", r.host)
	}

	loginMode := auth.OptionalLogin
	if r.serviceAccountJSONPath != "" {
		// if service account is specified, the request MUST be authenticated
		// otherwise it is optional.
		loginMode = auth.SilentLogin
	}
	authenticator := auth.NewAuthenticator(ctx, loginMode, auth.Options{
		ServiceAccountJSONPath: r.serviceAccountJSONPath,
	})

	httpClient, err := authenticator.Client()
	if err != nil {
		return nil, err
	}

	protocol := "https"
	if lhttp.IsLocalHost(r.host) {
		protocol = "http"
	}

	return &client{
		HTTP: httpClient,
		baseURL: &url.URL{
			Scheme: protocol,
			Host:   r.host,
			Path:   "/api/buildbucket/v1/",
		},
	}, nil
}

func (r *baseCommandRun) done(ctx context.Context, err error) int {
	if err != nil {
		logging.Errorf(ctx, "%s", err)
		return 1
	}
	return 0
}

// callAndDone makes a buildbucket API call, prints error or response, and returns exit code.
func (r *baseCommandRun) callAndDone(ctx context.Context, method, relURL string, body interface{}) int {
	client, err := r.createClient(ctx)
	if err != nil {
		return r.done(ctx, err)
	}

	res, err := client.call(ctx, method, relURL, body)
	if err != nil {
		return r.done(ctx, err)
	}

	fmt.Printf("%s\n", res)
	return 0
}

// buildIDArg can be embedded into a subcommand that accepts a build ID.
type buildIDArg struct {
	buildID int64
}

func (a *buildIDArg) parseArgs(args []string) error {
	if len(args) < 1 {
		return fmt.Errorf("missing parameter: <Build ID>")
	}
	if len(args) > 1 {
		return fmt.Errorf("unexpected arguments: %s", args[1:])
	}

	id, err := strconv.ParseInt(args[0], 10, 64)
	if err != nil {
		return fmt.Errorf("expected a build id (int64), got %s: %s", args[0], err)
	}

	a.buildID = id
	return nil
}
