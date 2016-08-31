// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package templates

import (
	"errors"
	"html/template"
	"io"

	"golang.org/x/net/context"
)

var errNoBundle = errors.New("templates: context doesn't have templates.Bundle")

type contextKey int

// Use replaces the template bundle in the context. There can be only one bundle
// installed at any time.
func Use(c context.Context, b *Bundle) context.Context {
	b.EnsureLoaded(c)
	return context.WithValue(c, contextKey(0), b)
}

// Get returns template from the currently loaded bundle or error if not found.
func Get(c context.Context, name string) (*template.Template, error) {
	if b, _ := c.Value(contextKey(0)).(*Bundle); b != nil {
		return b.Get(name)
	}
	return nil, errNoBundle
}

// Render finds top level template with given name and calls its Execute or
// ExecuteTemplate method (depending on the value of Bundle.DefaultTemplate).
//
// It always renders output into a byte buffer, to avoid partial results in case
// of errors.
func Render(c context.Context, name string, args Args) ([]byte, error) {
	if b, _ := c.Value(contextKey(0)).(*Bundle); b != nil {
		return b.Render(c, name, args)
	}
	return nil, errNoBundle
}

// MustRender renders the template into the output writer or panics.
//
// It never writes partial output. It also panics if attempt to write to
// the output fails.
func MustRender(c context.Context, out io.Writer, name string, args Args) {
	if b, _ := c.Value(contextKey(0)).(*Bundle); b != nil {
		b.MustRender(c, out, name, args)
		return
	}
	panic(errNoBundle)
}
