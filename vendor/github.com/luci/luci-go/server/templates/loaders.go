// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package templates

import (
	"html/template"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/net/context"
)

// AssetsLoader returns Loader that loads templates from the given assets map.
//
// The map is expected to have special structure: 'pages/' contain all top-level
// templates that will be loaded, 'includes/' contain templates that will be
// associated with every top-level template from 'pages/'.
//
// Only templates from 'pages/' are included in the output map.
func AssetsLoader(assets map[string]string) Loader {
	return func(c context.Context, funcMap template.FuncMap) (map[string]*template.Template, error) {
		// Pick all includes.
		includes := []string(nil)
		for name, body := range assets {
			if strings.HasPrefix(name, "includes/") {
				includes = append(includes, body)
			}
		}

		// Parse all top level templates and associate them with all includes.
		toplevel := map[string]*template.Template{}
		for name, body := range assets {
			if strings.HasPrefix(name, "pages/") {
				t, err := template.New(name).Funcs(funcMap).Parse(body)
				if err != nil {
					return nil, err
				}
				// TODO(vadimsh): There's probably a way to avoid reparsing includes
				// all the time.
				for _, includeSrc := range includes {
					if _, err := t.Parse(includeSrc); err != nil {
						return nil, err
					}
				}
				toplevel[name] = t
			}
		}

		return toplevel, nil
	}
}

// FileSystemLoader returns Loader that loads templates from file system.
//
// The directory with templates is expected to have special structure: 'pages/'
// contain all top-level templates that will be loaded, 'includes/' contain
// templates that will be associated with every top-level template from
// 'pages/'.
//
// Only templates from 'pages/' are included in the output map.
func FileSystemLoader(path string) Loader {
	return func(c context.Context, funcMap template.FuncMap) (map[string]*template.Template, error) {
		abs, err := filepath.Abs(path)
		if err != nil {
			return nil, err
		}

		// Read all relevant files into memory. It's ok, they are small.
		files := map[string]string{}
		if err = readFilesInDir(filepath.Join(abs, "includes"), files); err != nil {
			return nil, err
		}
		if err = readFilesInDir(filepath.Join(abs, "pages"), files); err != nil {
			return nil, err
		}

		// Convert to assets map as consumed by AssetsLoader (relative slash
		// separated paths) and reuse AssetsLoader.
		assets := map[string]string{}
		for k, v := range files {
			rel, err := filepath.Rel(abs, k)
			if err != nil {
				return nil, err
			}
			assets[filepath.ToSlash(rel)] = v
		}
		return AssetsLoader(assets)(c, funcMap)
	}
}

// readFilesInDir loads content of files into a map "file path" => content.
//
// Used only for small HTML templates, and thus it's fine to load them
// in memory.
//
// Does nothing is 'dir' is missing.
func readFilesInDir(dir string, out map[string]string) error {
	_, err := os.Stat(dir)
	if os.IsNotExist(err) {
		return nil
	}
	return filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return err
		}
		body, err := ioutil.ReadFile(path)
		if err == nil {
			out[path] = string(body)
		}
		return err
	})
}
