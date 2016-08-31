// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/luci/gae/service/datastore"
	"github.com/luci/luci-go/milo/api/resp"
	"github.com/luci/luci-go/milo/appengine/model"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/auth/identity"
	"github.com/luci/luci-go/server/auth/xsrf"
	"github.com/luci/luci-go/server/router"
	"golang.org/x/net/context"
)

type updateReq struct {
	Theme string
}

// GetTheme returns the chosen theme based on the current user.
func GetTheme(c context.Context, r *http.Request) Theme {
	cfg := getUserSettings(c)
	if cfg == nil {
		cfg = getCookieSettings(c, r)
	}
	if t, ok := Themes[cfg.Theme]; ok {
		return t
	}
	return Default
}

func getUserSettings(c context.Context) *model.UserConfig {
	// First get settings
	cu := auth.CurrentUser(c)
	if cu.Identity == identity.AnonymousIdentity {
		return nil
	}
	ds := datastore.Get(c)
	userSettings := &model.UserConfig{UserID: cu.Identity}
	ds.Get(userSettings)
	// Even if the get fails (No user found) we still want to return an empty
	// UserConfig with defaults.
	return userSettings
}

// getCookieSettings returns user settings from a cookie, or a blank slate with
// defaults if no settings were found.
func getCookieSettings(c context.Context, r *http.Request) *model.UserConfig {
	cookie, err := r.Cookie("luci-milo")
	config := model.UserConfig{
		UserID: identity.AnonymousIdentity,
		Theme:  Default.Name,
	}
	if err != nil {
		return &config
	}
	// If this errors, then just return the default.
	s, err := base64.StdEncoding.DecodeString(cookie.Value)
	if err != nil {
		panic(err)
	}
	err = json.Unmarshal([]byte(s), &config)
	if err != nil {
		panic(err)
	}
	return &config
}

// ChangeSettings is invoked in a POST request to settings and changes either
// the user settings in the datastore, or the cookies if user is anon.
func ChangeSettings(ctx *router.Context) {
	c, h, r := ctx.Context, ctx.Writer, ctx.Request

	// First, check XSRF token.
	err := xsrf.Check(c, r.FormValue("xsrf_token"))
	if err != nil {
		h.WriteHeader(http.StatusUnauthorized)
		h.Write([]byte("Failed XSRF check."))
		return
	}

	u := &updateReq{
		Theme: r.FormValue("theme"),
	}
	validateUpdate(u)
	s := getUserSettings(c)
	if s == nil {
		// User doesn't exist, just respond with a cookie.
		s = getCookieSettings(c, r)
		s.Theme = u.Theme
		setCookieSettings(h, s)
	} else {
		changeUserSettings(c, u)
	}

	// Redirect to the GET endpoint.
	http.Redirect(h, r, r.URL.String(), http.StatusSeeOther)
}

// setCookieSettings sets the cfg object as a base64 json serialized string.
func setCookieSettings(h http.ResponseWriter, cfg *model.UserConfig) {
	s, err := json.Marshal(cfg)
	if err != nil {
		panic(err)
	}
	bs := base64.StdEncoding.EncodeToString(s)
	cookie := http.Cookie{
		Name:  "luci-milo",
		Value: bs,
	}
	http.SetCookie(h, &cookie)
}

func validateUpdate(u *updateReq) error {
	if _, ok := Themes[u.Theme]; ok {
		return nil
	}
	return fmt.Errorf("Invalid theme %s", u.Theme)
}

func changeUserSettings(c context.Context, u *updateReq) error {
	cfg := getUserSettings(c)
	err := validateUpdate(u)
	if err != nil {
		return err
	}
	ds := datastore.Get(c)
	return ds.Put(cfg)
}

func getSettings(c context.Context, r *http.Request) (*resp.Settings, error) {
	userSettings := getUserSettings(c)
	if userSettings == nil {
		userSettings = getCookieSettings(c, r)
	}

	result := &resp.Settings{}
	result.ActionURL = r.URL.String()
	result.Theme = &resp.Choices{
		Choices:  GetAllThemes(),
		Selected: userSettings.Theme,
	}

	return result, nil
}
