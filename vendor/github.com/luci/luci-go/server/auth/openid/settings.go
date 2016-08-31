// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package openid

import (
	"errors"
	"html/template"

	"golang.org/x/net/context"

	"github.com/luci/luci-go/server/settings"
)

// SettingsKey is key for OpenID settings (described by Settings struct) in
// settings store. See github.com/luci/luci-go/server/settings.
const SettingsKey = "openid_auth"

var (
	// ErrNotConfigured is returned by various functions if OpenID settings are
	// not properly configured.
	ErrNotConfigured = errors.New("openid: not configured")
)

// Settings contain parameters of OpenID protocol. They are stored in app
// settings store under SettingsKey key.
type Settings struct {
	// DiscoveryURL is where to grab discovery document with provider's config.
	// Use `https://accounts.google.com/.well-known/openid-configuration` for
	// Google OpenID Connect provider.
	DiscoveryURL string `json:"discovery_url"`

	// ClientID identifies OAuth2 Web client representing the application. Create
	// one in Cloud Console if using Google OpenID Connect provider.
	ClientID string `json:"client_id"`

	// ClientSecret is a secret associated with ClientID.
	ClientSecret string `json:"client_secret"`

	// RedirectURI must be `https://<apphost>/auth/openid/callback`. It is stored
	// in config explicitly to remind admin that OAuth2 client in Cloud Console
	// must be configured accordingly.
	RedirectURI string `json:"redirect_uri"`
}

// fetchCachedSettings fetches OpenID configuration from the settings store.
func fetchCachedSettings(c context.Context) (*Settings, error) {
	cfg := &Settings{}
	if err := settings.Get(c, SettingsKey, cfg); err != settings.ErrNoSettings {
		return cfg, err
	}
	return cfg, nil
}

////////////////////////////////////////////////////////////////////////////////
// UI for configuring OpenID.

type settingsUIPage struct {
	settings.BaseUIPage
}

func (settingsUIPage) Title(c context.Context) (string, error) {
	return "OpenID authentication settings", nil
}

func (settingsUIPage) Overview(c context.Context) (template.HTML, error) {
	return `<p>OpenID Connect is the primary authentication protocol used by
luci services to authenticate web users. It is triggered whenever incoming HTTP
request doesn't have a session cookie set or the session has expired.</p>

<p>It needs to be configured with details of what OpenID provider to use and how
to authenticate calls to it.</p>`, nil
}

func (settingsUIPage) Fields(c context.Context) ([]settings.UIField, error) {
	return []settings.UIField{
		{
			ID:    "DiscoveryURL",
			Title: "Discovery URL",
			Type:  settings.UIFieldText,
			Help: `Where to grab OpenID Connect discovery document with provider's
config. Use <b>https://accounts.google.com/.well-known/openid-configuration</b>
for Google OpenID Connect provider.`,
		},
		{
			ID:    "ClientID",
			Title: "OAuth client ID",
			Type:  settings.UIFieldText,
			Help: `Identifies OAuth2 Web Client representing the application.
Create one in <a href="https://console.developers.google.com">Cloud Console</a>
if using Google OpenID Connect provider. It is fine to reuse an existing OAuth2
client as long as you register additional redirect URI in its configuration.`,
		},
		{
			ID:    "ClientSecret",
			Title: "OAuth client secret",
			Type:  settings.UIFieldText,
			Help: `Secret associated with OAuth2 Web Client. Grab it from
<a href="https://console.developers.google.com">Cloud Console</a>.`,
		},
		{
			ID:    "RedirectURI",
			Title: "Redirect URI",
			Type:  settings.UIFieldText,
			Help: `OpenID callback URI that must be set to
<b>https://<i>your-host</i>/auth/openid/callback</b>. Configure OAuth2 Web
Client with exact same value.`,
		},
	}, nil
}

func (settingsUIPage) ReadSettings(c context.Context) (map[string]string, error) {
	s := Settings{}
	err := settings.GetUncached(c, SettingsKey, &s)
	if err != nil && err != settings.ErrNoSettings {
		return nil, err
	}
	return map[string]string{
		"DiscoveryURL": s.DiscoveryURL,
		"ClientID":     s.ClientID,
		"ClientSecret": s.ClientSecret,
		"RedirectURI":  s.RedirectURI,
	}, nil
}

func (settingsUIPage) WriteSettings(c context.Context, values map[string]string, who, why string) error {
	return settings.SetIfChanged(c, SettingsKey, &Settings{
		DiscoveryURL: values["DiscoveryURL"],
		ClientID:     values["ClientID"],
		ClientSecret: values["ClientSecret"],
		RedirectURI:  values["RedirectURI"],
	}, who, why)
}

func init() {
	settings.RegisterUIPage(SettingsKey, settingsUIPage{})
}
