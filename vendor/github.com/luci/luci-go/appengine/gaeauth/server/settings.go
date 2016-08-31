// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package server

import (
	"fmt"
	"html/template"
	"net/url"

	"golang.org/x/net/context"

	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/appengine/gaeauth/server/internal/authdbimpl"
	"github.com/luci/luci-go/server/settings"
)

type settingsUIPage struct {
	settings.BaseUIPage
}

func (settingsUIPage) Title(c context.Context) (string, error) {
	return "Authorization settings", nil
}

func (settingsUIPage) Overview(c context.Context) (template.HTML, error) {
	serviceAcc, err := info.Get(c).ServiceAccount()
	if err != nil {
		return "", err
	}
	return template.HTML(fmt.Sprintf(`<p>Prerequisites for fetching groups from an
<a href="https://github.com/luci/luci-py/tree/master/appengine/auth_service">auth service</a>:</p>
<ul>
  <li>
    Google Cloud Pub/Sub service is enabled in the Cloud Console project.
  </li>
  <li>
    The <b>auth-trusted-services</b> group on the auth service includes
    the service account belonging to this application:
    <b>%s</b>.
  </li>
</ul>`, template.HTMLEscapeString(serviceAcc))), nil
}

func (settingsUIPage) Fields(c context.Context) ([]settings.UIField, error) {
	return []settings.UIField{
		{
			ID:    "AuthServiceURL",
			Title: "Auth service URL",
			Type:  settings.UIFieldText,
			Validator: func(authServiceURL string) error {
				if authServiceURL != "" {
					parsed, err := url.Parse(authServiceURL)
					if err != nil {
						return fmt.Errorf("bad URL %q - %s", authServiceURL, err)
					}
					if !parsed.IsAbs() || parsed.Path != "" {
						return fmt.Errorf("bad URL %q - must be host root URL", authServiceURL)
					}
				}
				return nil
			},
		},
		{
			ID:    "LatestRev",
			Title: "Latest fetched revision",
			Type:  settings.UIFieldStatic,
		},
	}, nil
}

func (settingsUIPage) ReadSettings(c context.Context) (map[string]string, error) {
	switch info, err := authdbimpl.GetLatestSnapshotInfo(c); {
	case err != nil:
		return nil, err
	case info == nil:
		return map[string]string{
			"AuthServiceURL": "",
			"LatestRev":      "unknown (not configured)",
		}, nil
	default:
		return map[string]string{
			"AuthServiceURL": info.AuthServiceURL,
			"LatestRev":      fmt.Sprintf("%d", info.Rev),
		}, nil
	}
}

func (settingsUIPage) WriteSettings(c context.Context, values map[string]string, who, why string) error {
	authServiceURL := values["AuthServiceURL"]
	baseURL := "https://" + info.Get(c).DefaultVersionHostname()
	return authdbimpl.ConfigureAuthService(c, baseURL, authServiceURL)
}

func init() {
	settings.RegisterUIPage("auth_service", settingsUIPage{})
}
