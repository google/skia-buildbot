// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package signing

import (
	"time"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/common/data/caching/proccache"
	"github.com/luci/luci-go/server/auth/internal"
)

// ServiceInfo describes identity of some service.
//
// It matches JSON format of /auth/api/v1/server/info endpoint.
type ServiceInfo struct {
	AppID              string `json:"app_id,omitempty"`
	AppRuntime         string `json:"app_runtime,omitempty"`
	AppRuntimeVersion  string `json:"app_runtime_version,omitempty"`
	AppVersion         string `json:"app_version,omitempty"`
	ServiceAccountName string `json:"service_account_name,omitempty"`
}

type serviceInfoKey string

// FetchServiceInfo fetches information about the service from the given URL.
//
// The server is expected to reply with JSON described by ServiceInfo struct
// (like LUCI services do). Uses proccache to cache the response for 1h.
//
// LUCI services serve certificates at /auth/api/v1/server/info.
func FetchServiceInfo(c context.Context, url string) (*ServiceInfo, error) {
	info, err := proccache.GetOrMake(c, serviceInfoKey(url), func() (interface{}, time.Duration, error) {
		info := &ServiceInfo{}
		req := internal.Request{
			Method: "GET",
			URL:    url,
			Out:    info,
		}
		if err := req.Do(c); err != nil {
			return nil, 0, err
		}
		return info, time.Hour, nil
	})
	if err != nil {
		return nil, err
	}
	return info.(*ServiceInfo), nil
}

// FetchServiceInfoFromLUCIService is shortcut for FetchServiceInfo that uses
// LUCI-specific endpoint.
//
// 'serviceURL' is root URL of the service (e.g. 'https://example.com').
func FetchServiceInfoFromLUCIService(c context.Context, serviceURL string) (*ServiceInfo, error) {
	return FetchServiceInfo(c, serviceURL+"/auth/api/v1/server/info")
}
