// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package remote

import (
	"encoding/base64"
	"fmt"
	"net/http"
	"net/url"

	"golang.org/x/net/context"
	"google.golang.org/api/googleapi"

	configApi "github.com/luci/luci-go/common/api/luci_config/config/v1"
	"github.com/luci/luci-go/common/config"
	"github.com/luci/luci-go/common/errors"
	"github.com/luci/luci-go/common/logging"
)

// ClientFactory returns HTTP client to use (given a context).
//
// See 'New' for more details.
type ClientFactory func(context.Context) (*http.Client, error)

// New returns an implementation of the config service which talks to the actual
// luci-config service using given transport.
//
// configServiceURL is usually "https://<host>/_ah/api/config/v1/".
//
// ClientFactory returns http.Clients to use for requests (given incoming
// contexts). It's required mostly to support GAE environment, where round
// trippers are bound to contexts and carry RPC deadlines.
//
// If 'clients' is nil, http.DefaultClient will be used for all requests.
func New(configServiceURL string, clients ClientFactory) config.Interface {
	serviceURL, err := url.Parse(configServiceURL)
	if err != nil {
		panic(fmt.Errorf("failed to parse service URL: %v", err))
	}
	if clients == nil {
		clients = func(context.Context) (*http.Client, error) {
			return http.DefaultClient, nil
		}
	}
	return &remoteImpl{
		serviceURL: serviceURL,
		clients:    clients,
	}
}

type remoteImpl struct {
	serviceURL *url.URL
	clients    ClientFactory
}

// service returns Cloud Endpoints API client bound to the given context.
//
// It inherits context's deadline and transport.
func (r *remoteImpl) service(ctx context.Context) (*configApi.Service, error) {
	client, err := r.clients(ctx)
	if err != nil {
		return nil, err
	}
	service, err := configApi.New(client)
	if err != nil {
		return nil, err
	}
	service.BasePath = r.serviceURL.String()
	return service, nil
}

func (r *remoteImpl) ServiceURL(ctx context.Context) url.URL {
	return *r.serviceURL
}

func (r *remoteImpl) GetConfig(ctx context.Context, configSet, path string, hashOnly bool) (*config.Config, error) {
	srv, err := r.service(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := srv.GetConfig(configSet, path).HashOnly(hashOnly).Context(ctx).Do()
	if err != nil {
		return nil, apiErr(err)
	}

	var decoded []byte
	if !hashOnly {
		decoded, err = base64.StdEncoding.DecodeString(resp.Content)
		if err != nil {
			return nil, err
		}
	}

	return &config.Config{
		ConfigSet:   configSet,
		Path:        path,
		Content:     string(decoded),
		ContentHash: resp.ContentHash,
		Revision:    resp.Revision,
	}, nil
}

func (r *remoteImpl) GetConfigByHash(ctx context.Context, configSet string) (string, error) {
	srv, err := r.service(ctx)
	if err != nil {
		return "", err
	}

	resp, err := srv.GetConfigByHash(configSet).Context(ctx).Do()
	if err != nil {
		return "", apiErr(err)
	}

	decoded, err := base64.StdEncoding.DecodeString(resp.Content)
	if err != nil {
		return "", err
	}

	return string(decoded), nil
}

func (r *remoteImpl) GetConfigSetLocation(ctx context.Context, configSet string) (*url.URL, error) {
	if configSet == "" {
		return nil, fmt.Errorf("configSet must be a non-empty string")
	}

	srv, err := r.service(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := srv.GetMapping().ConfigSet(configSet).Context(ctx).Do()
	if err != nil {
		return nil, apiErr(err)
	}

	urlString := "sentinel"
	for _, mapping := range resp.Mappings {
		if mapping.ConfigSet == configSet {
			if urlString != "sentinel" {
				return nil, fmt.Errorf(
					"duplicate entries %q and %q for location of config set %s",
					urlString, mapping.Location, configSet)
			}

			urlString = mapping.Location
		}
	}

	url, err := url.Parse(urlString)
	if err != nil {
		return nil, err
	}

	return url, nil
}

func (r *remoteImpl) GetProjects(ctx context.Context) ([]config.Project, error) {
	srv, err := r.service(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := srv.GetProjects().Context(ctx).Do()
	if err != nil {
		return nil, apiErr(err)
	}

	projects := make([]config.Project, len(resp.Projects))
	for i, p := range resp.Projects {
		repoType := parseWireRepoType(p.RepoType)

		url, err := url.Parse(p.RepoUrl)
		if err != nil {
			lc := logging.SetField(ctx, "projectID", p.Id)
			logging.Warningf(lc, "Failed to parse repo URL %q: %s", p.RepoUrl, err)
		}

		projects[i] = config.Project{
			p.Id,
			p.Name,
			repoType,
			url,
		}
	}
	return projects, err
}

func (r *remoteImpl) GetProjectConfigs(ctx context.Context, path string, hashesOnly bool) ([]config.Config, error) {
	srv, err := r.service(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := srv.GetProjectConfigs(path).HashesOnly(hashesOnly).Context(ctx).Do()
	if err != nil {
		return nil, apiErr(err)
	}

	c := logging.SetField(ctx, "path", path)
	return convertMultiWireConfigs(c, path, resp, hashesOnly)
}

func (r *remoteImpl) GetRefConfigs(ctx context.Context, path string, hashesOnly bool) ([]config.Config, error) {
	srv, err := r.service(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := srv.GetRefConfigs(path).HashesOnly(hashesOnly).Context(ctx).Do()
	if err != nil {
		return nil, apiErr(err)
	}

	c := logging.SetField(ctx, "path", path)
	return convertMultiWireConfigs(c, path, resp, hashesOnly)
}

func (r *remoteImpl) GetRefs(ctx context.Context, projectID string) ([]string, error) {
	srv, err := r.service(ctx)
	if err != nil {
		return nil, err
	}

	resp, err := srv.GetRefs(projectID).Context(ctx).Do()
	if err != nil {
		return nil, apiErr(err)
	}

	refs := make([]string, len(resp.Refs))
	for i, ref := range resp.Refs {
		refs[i] = ref.Name
	}
	return refs, err
}

// convertMultiWireConfigs is a utility to convert what we get over the wire
// into the structs we use in the config package.
func convertMultiWireConfigs(ctx context.Context, path string, wireConfigs *configApi.LuciConfigGetConfigMultiResponseMessage, hashesOnly bool) ([]config.Config, error) {
	configs := make([]config.Config, len(wireConfigs.Configs))
	for i, c := range wireConfigs.Configs {
		var decoded []byte
		var err error

		if !hashesOnly {
			decoded, err = base64.StdEncoding.DecodeString(c.Content)
			if err != nil {
				lc := logging.SetField(ctx, "configSet", c.ConfigSet)
				logging.Warningf(lc, "Failed to base64 decode config: %s", err)
			}
		}

		configs[i] = config.Config{
			ConfigSet:   c.ConfigSet,
			Path:        path,
			Error:       err,
			Content:     string(decoded),
			ContentHash: c.ContentHash,
			Revision:    c.Revision,
		}
	}

	return configs, nil
}

// parseWireRepoType parses the string received over the wire from
// the luci-config service that represents the repo type.
func parseWireRepoType(s string) config.RepoType {
	if s == string(config.GitilesRepo) {
		return config.GitilesRepo
	}

	return config.UnknownRepo
}

// apiErr converts googleapi.Error to an appropriate type.
func apiErr(e error) error {
	err, ok := e.(*googleapi.Error)
	if !ok {
		return e
	}
	if err.Code == 404 {
		return config.ErrNoConfig
	}
	if err.Code >= 500 {
		return errors.WrapTransient(err)
	}
	return err
}
