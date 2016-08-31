// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package templates

import (
	"bytes"
	"fmt"
	"html/template"
	"io"
	"sync"

	"golang.org/x/net/context"
)

// Loader knows how to load template sets.
type Loader func(context.Context, template.FuncMap) (map[string]*template.Template, error)

// Args contains data passed to the template.
type Args map[string]interface{}

// MergeArgs combines multiple Args instances into one. Returns nil if all
// passed args are empty.
func MergeArgs(args ...Args) Args {
	total := 0
	for _, a := range args {
		total += len(a)
	}
	if total == 0 {
		return nil
	}
	res := make(Args, total)
	for _, a := range args {
		for k, v := range a {
			res[k] = v
		}
	}
	return res
}

// Bundle is a bunch of templates lazily loaded at the same time. They may share
// associated templates. Bundle is injected into the context.
type Bundle struct {
	// Loader will be called once to attempt to load templates on the first use.
	//
	// There are some predefined loaders you can use, see AssetsLoader(...)
	// for example.
	Loader Loader

	// DebugMode can be set to true to enable template reloading before each use.
	//
	// It disables the caching of compiled templates, essentially. Useful during
	// development.
	DebugMode bool

	// FuncMap contains functions accessible from templates.
	//
	// Will be passed to Loader on first use. Not used after that.
	FuncMap template.FuncMap

	// DefaultTemplate is a name of subtemplate to pass to ExecuteTemplate when
	// rendering a template via Render(...) or MustRender(...).
	//
	// For example, if all templates in a bundle are built around some base
	// template (that defined structure of the page), DefaultTemplate can be set
	// to the name of that base template.
	//
	// If DefaultTemplate is empty, Render(...) will use Execute(...) instead of
	// ExecuteTemplate(...).
	DefaultTemplate string

	// DefaultArgs generates default arguments to use when rendering templates.
	//
	// Additional arguments passed to Render will be merged on top of the
	// default ones. DefaultArgs is called each time Render is called.
	DefaultArgs func(c context.Context) (Args, error)

	once      sync.Once
	templates map[string]*template.Template // result of call to Loader(...)
	err       error                         // error from Loader, if any
}

// EnsureLoaded loads all the templates if they haven't been loaded yet.
func (b *Bundle) EnsureLoaded(c context.Context) error {
	// Always reload in debug mode. Load only once in non-debug mode.
	if b.DebugMode {
		b.templates, b.err = b.Loader(c, b.FuncMap)
	} else {
		b.once.Do(func() {
			b.templates, b.err = b.Loader(c, b.FuncMap)
		})
	}
	return b.err
}

// Get returns the loaded template given its name or error if not found.
//
// The bundle must be loaded by this point (via call to EnsureLoaded).
func (b *Bundle) Get(name string) (*template.Template, error) {
	if b.err != nil {
		return nil, b.err
	}
	if templ := b.templates[name]; templ != nil {
		return templ, nil
	}
	return nil, fmt.Errorf("template: no such template %q in the bundle", name)
}

// Render finds template with given name and calls its Execute or
// ExecuteTemplate method (depending on the value of DefaultTemplate).
//
// It always renders output into byte buffer, to avoid partial results in case
// of errors.
//
// The bundle must be loaded by this point (via call to EnsureLoaded).
func (b *Bundle) Render(c context.Context, name string, args Args) ([]byte, error) {
	templ, err := b.Get(name)
	if err != nil {
		return nil, err
	}

	var defArgs Args
	if b.DefaultArgs != nil {
		var err error
		if defArgs, err = b.DefaultArgs(c); err != nil {
			return nil, err
		}
	}

	out := bytes.Buffer{}
	if b.DefaultTemplate == "" {
		err = templ.Execute(&out, MergeArgs(defArgs, args))
	} else {
		err = templ.ExecuteTemplate(&out, b.DefaultTemplate, MergeArgs(defArgs, args))
	}
	if err != nil {
		return nil, err
	}

	return out.Bytes(), nil
}

// MustRender renders the template into the output writer or panics.
//
// It never writes partial output. It also panics if attempt to write to
// the output fails.
//
// The bundle must be loaded by this point (via call to EnsureLoaded).
func (b *Bundle) MustRender(c context.Context, out io.Writer, name string, args Args) {
	blob, err := b.Render(c, name, args)
	if err != nil {
		panic(err)
	}
	_, err = out.Write(blob)
	if err != nil {
		panic(err)
	}
}
