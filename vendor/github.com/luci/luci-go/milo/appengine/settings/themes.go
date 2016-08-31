// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"fmt"
	"io"
	"net/http"
	"path"
	"sort"
	"strings"

	"google.golang.org/appengine"

	"github.com/julienschmidt/httprouter"
	"github.com/luci/gae/service/info"
	"github.com/luci/luci-go/appengine/gaeauth/server"
	"github.com/luci/luci-go/appengine/gaemiddleware"
	"github.com/luci/luci-go/common/clock"
	"github.com/luci/luci-go/milo/common/miloerror"
	"github.com/luci/luci-go/server/analytics"
	"github.com/luci/luci-go/server/auth"
	"github.com/luci/luci-go/server/router"
	"github.com/luci/luci-go/server/templates"
	"golang.org/x/net/context"
)

type themeContextKey string

// NamedBundle is a tuple of a name (That matches it's corresponding theme)
// and a template bundle.
type NamedBundle struct {
	Name   string
	Bundle *templates.Bundle
	Theme  *Theme
}

// Theme is the base type for specifying where to find a Theme.
type Theme struct {
	// IsTemplate is true if this theme is a Go template type template, and false
	// if it is a client side (eg. Polymer) type template.
	IsTemplate bool
	// Name is the name of the Theme.
	Name string
}

// ThemedHandler is the base type for any milo html handlers.
type ThemedHandler interface {
	// Return the template name given a theme name.
	GetTemplateName(Theme) string
	// Render the page for server side renders.
	Render(context.Context, *http.Request, httprouter.Params) (*templates.Args, error)
}

var (
	// Default is the global default theme for anonomyous users.
	Default = Theme{IsTemplate: true, Name: "buildbot"}
	// Themes is a list of all known themes.
	Themes = map[string]Theme{
		"buildbot":  Default,
		"bootstrap": {IsTemplate: true, Name: "bootstrap"},
	}
)

// GetAllThemes gets all known themes as a list of strings.
func GetAllThemes() []string {
	results := make([]string, 0, len(Themes))
	for k := range Themes {
		results = append(results, k)
	}
	sort.Strings(results)
	return results
}

// GetTemplateBundles is used to render HTML templates. It provides a base args
// passed to all templates.
func GetTemplateBundles() []NamedBundle {
	result := []NamedBundle{}
	for name, t := range Themes {
		if t.IsTemplate {
			templateBundle := &templates.Bundle{
				Loader:          templates.FileSystemLoader(path.Join("templates", name)),
				DefaultTemplate: name,
				DebugMode:       appengine.IsDevAppServer(),
				DefaultArgs: func(c context.Context) (templates.Args, error) {
					loginURL, err := auth.LoginURL(c, "/")
					if err != nil {
						return nil, err
					}
					logoutURL, err := auth.LogoutURL(c, "/")
					if err != nil {
						return nil, err
					}
					if err != nil {
						return nil, err
					}
					return templates.Args{
						"AppVersion":  strings.Split(info.Get(c).VersionID(), ".")[0],
						"IsAnonymous": auth.CurrentIdentity(c) == "anonymous:anonymous",
						"User":        auth.CurrentUser(c),
						"LoginURL":    loginURL,
						"LogoutURL":   logoutURL,
						"CurrentTime": clock.Now(c),
						"Analytics":   analytics.Snippet(c),
					}, nil
				},
				FuncMap: funcMap,
			}
			result = append(result, NamedBundle{name, templateBundle, &t})
		}
	}
	return result
}

// UseNamedBundle is like templates.Use, but with the choice of one of many bundles (themes)
func UseNamedBundle(c context.Context, nb NamedBundle) (context.Context, error) {
	err := nb.Bundle.EnsureLoaded(c)
	return context.WithValue(c, themeContextKey(nb.Name), nb.Bundle), err
}

// withNamedBundle is like templates.WithTemplates, but with the choice of one of many bundles (themes)
func withNamedBundle(nb NamedBundle) router.Middleware {
	return func(c *router.Context, next router.Handler) {
		var err error
		c.Context, err = UseNamedBundle(c.Context, nb) // calls EnsureLoaded and initializes b.err inside
		if err != nil {
			http.Error(c.Writer, fmt.Sprintf("Can't load HTML templates.\n%s", err), http.StatusInternalServerError)
			return
		}
		next(c)
	}
}

// themedMustRender renders theme and panics if it can't be rendered.  This should never fail in
// production.
func themedMustRender(c context.Context, out io.Writer, theme, name string, args templates.Args) {
	if b, _ := c.Value(themeContextKey(theme)).(*templates.Bundle); b != nil {
		blob, err := b.Render(c, name, args)
		if err != nil {
			panic(fmt.Errorf("Could not render template %s from theme %s:\n%s", name, theme, err))
		}
		_, err = out.Write(blob)
		if err != nil {
			panic(fmt.Errorf("Could not write out template %s from theme %s:\n%s", name, theme, err))
		}
		return
	}
	panic(fmt.Errorf("Error: Could not load template %s from theme %s", name, theme))
}

// Base returns the basic luci appengine middlewares.
func Base() router.MiddlewareChain {
	methods := auth.Authenticator{
		&server.OAuth2Method{Scopes: []string{server.EmailScope}},
		server.CookieAuth,
		&server.InboundAppIDAuthMethod{},
	}
	m := gaemiddleware.BaseProd().Extend(auth.Use(methods), auth.Authenticate)
	for _, nb := range GetTemplateBundles() {
		m = m.Extend(withNamedBundle(nb))
	}
	return m
}

// Wrap adapts a ThemedHandler into a router.Handler. Of note, the
// Render functions' interface into rendering is purely through a single
// templates.Args value which gets rendered here, while the http.ResponseWriter
// is stripped out.
func Wrap(h ThemedHandler) router.Handler {
	return func(c *router.Context) {
		// Figure out if we need to do the things.
		theme := GetTheme(c.Context, c.Request)
		template := h.GetTemplateName(theme)

		// Do the things.
		args, err := h.Render(c.Context, c.Request, c.Params)

		// Throw errors.
		// TODO(hinoka): Add themes and templates for errors so they look better.
		if err != nil {
			if merr, ok := err.(*miloerror.Error); ok {
				http.Error(c.Writer, merr.Message, merr.Code)
			} else {
				http.Error(c.Writer, err.Error(), http.StatusInternalServerError)
			}
			return
		}

		// Render the stuff.
		name := fmt.Sprintf("pages/%s", template)
		themedMustRender(c.Context, c.Writer, theme.Name, name, *args)
	}
}
