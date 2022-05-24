package imports

/*
	Utilities for finding import relationships between Go packages.
*/

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/golang"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

var (
	allPkgData    map[string]*Package
	cachedPkgData = map[string]*Package{}
)

// Package contains information about a Go package.
// Ideally we would just reuse cmd/go/internal/load.Package, but we're not
// allowed to import internal packages. So we've copy/pasted the relevant
// parts of that struct below.
type Package struct {
	Dir           string   `json:",omitempty"` // directory containing package sources
	ImportPath    string   `json:",omitempty"` // import path of package in dir
	ImportComment string   `json:",omitempty"` // path in import comment on package statement
	Name          string   `json:",omitempty"` // package name
	Doc           string   `json:",omitempty"` // package documentation string
	Target        string   `json:",omitempty"` // installed target for this package (may be executable)
	ForTest       string   `json:",omitempty"` // package is only for use in named test
	Export        string   `json:",omitempty"` // file containing export data (set by go list -export)
	Match         []string `json:",omitempty"` // command-line patterns matching this package
	Standard      bool     `json:",omitempty"` // is this package part of the standard Go library?

	// Dependency information
	Imports []string `json:",omitempty"` // import paths used by this package
	Deps    []string `json:",omitempty"` // all (recursively) imported dependencies
}

// pkgDataWriter is an io.Writer which reads package data from "go list".
type pkgDataWriter struct {
	buf   []byte
	idx   int
	cb    func(*Package)
	total int
}

// See documentation for io.Writer.
func (w *pkgDataWriter) Write(b []byte) (int, error) {
	// Unfortunately, when listing multiple packages, "go list" doesn't
	// return valid JSON. Instead, it returns one JSON dict for each
	// package, separated by newline. We have to look for "\n}" to mark
	// the end of each package.
	marker := "\n}"
	newBuf := append(w.buf, b...) // Combine new data with existing data.
	newStartIdx := 0              // Start index of the next package in the buffer.

	// Loop through the new bytes looking for the marker string. When we
	// find it, we can slice the buffer from newStartIdx to idx to find the
	// complete data for the current package, parse it as JSON, and run the
	// callback function.
	for idx := len(w.buf); idx < len(newBuf); idx++ {
		if idx >= len(marker) && string(newBuf[idx-len(marker):idx]) == marker {
			// We've found a complete package description. Parse
			// it as JSON and run the callback function.
			var pkg Package
			slice := newBuf[newStartIdx:idx]
			if err := json.Unmarshal(slice, &pkg); err != nil {
				sklog.Errorf("Error parsing JSON from output: %s", err)
				// Return the number of bytes we read which did
				// not contain an error, ie. everything up to
				// newStartIdx.
				read := 0
				if newStartIdx > len(w.buf) {
					read = newStartIdx - len(w.buf)
				}
				return read, err
			}
			w.cb(&pkg)

			// Bump the newStartIdx to the current idx to mark the
			// start of the next package.
			newStartIdx = idx
		}
	}
	w.buf = newBuf[newStartIdx:]
	w.total += len(b)
	return len(b), nil
}

func newPkgDataWriter(cb func(*Package)) io.Writer {
	return &pkgDataWriter{
		buf: []byte{},
		cb:  cb,
	}
}

// getPackageData is a helper function which returns data for the given
// package(s). Returns a map to facilitate searching for multiple packages in
// the same call to "go list", eg. "go.skia.org/...", which is much faster.
func getPackageData(ctx context.Context, name string) (map[string]*Package, error) {
	// Return the cached data, if it exists.
	if pkg, ok := cachedPkgData[name]; ok {
		return map[string]*Package{
			name: pkg,
		}, nil
	}

	// Run "go list" to obtain information about the given package(s).
	pkgs := map[string]*Package{}
	goBin, err := golang.FindGo()
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	cmd := &exec.Command{
		Name: goBin,
		Args: []string{"list", "--json", name},
		Stdout: newPkgDataWriter(func(pkg *Package) {
			pkgs[pkg.ImportPath] = pkg
		}),
	}
	if err := exec.Run(ctx, cmd); err != nil {
		return nil, err
	}
	// Cache the returned data.
	for k, v := range pkgs {
		cachedPkgData[k] = v
	}
	return pkgs, nil
}

// GetPackageData returns information about the given package.
func GetPackageData(ctx context.Context, name string) (*Package, error) {
	pkgs, err := getPackageData(ctx, name)
	if err != nil {
		return nil, err
	}
	if len(pkgs) != 1 {
		return nil, fmt.Errorf("Found multiple entries for %s: %v", name, pkgs)
	}
	for _, pkg := range pkgs {
		return pkg, nil
	}
	return nil, errors.New("Shouldn't hit this case.")
}

// LoadAllPackageData obtains information about all packages under
// go.skia.org/infra/... and caches it, returning it for convenience.
func LoadAllPackageData(ctx context.Context) (map[string]*Package, error) {
	// In addition to maintaining the cache for individual packages, we
	// cache the result of this function for convenience.
	if allPkgData != nil {
		return allPkgData, nil
	}
	allPkgs, err := getPackageData(ctx, "go.skia.org/infra/...")
	if err != nil {
		return nil, err
	}
	allPkgData = allPkgs
	return allPkgs, nil
}

// IsBuiltIn returns true if the given package name looks like a built-in
// package.
func IsBuiltIn(pkgName string) bool {
	// This is kind of a hack, but it works in practice: builtin packages
	// do not have a dot in their first path component, while others do.
	// TODO(borenet): We could probably use Package.Standard instead, but
	// that would require running "go list" to obtain data for the standard
	// packages as well.
	return !strings.Contains(strings.SplitN(pkgName, "/", 1)[0], ".")
}

// FindImportPaths returns a slice of slices indicating the import paths from
// one package to another.
func FindImportPaths(ctx context.Context, startPkg, findPkg string) ([][]string, error) {
	// Cache values for each package to prevent repeating the same work.
	cache := map[string][][]string{}

	var helper func(string) ([][]string, error)
	helper = func(currentPkg string) ([][]string, error) {
		// Returned the cached value, if any.
		if rv, ok := cache[currentPkg]; ok {
			return rv, nil
		}

		// Find the imports for startPkg.
		data, err := GetPackageData(ctx, currentPkg)
		if err != nil {
			return nil, err
		}
		foundPaths := [][]string{}
		for _, imp := range data.Imports {
			if imp == findPkg {
				foundPaths = append(foundPaths, []string{findPkg})
			} else if !IsBuiltIn(imp) {
				// Recursively search non-built-in packages.
				recFoundPaths, err := helper(imp)
				if err != nil {
					return nil, err
				}
				foundPaths = append(foundPaths, recFoundPaths...)
			}
		}
		for idx := range foundPaths {
			foundPaths[idx] = append(foundPaths[idx], currentPkg)
		}

		// Cache the return value.
		cache[currentPkg] = foundPaths

		return foundPaths, nil
	}
	return helper(startPkg)
}

// FindImporters returns a slice of package names indicating which packages
// under go.skia.org/infra/... directly import the given package.
func FindImporters(ctx context.Context, findPkg string) ([]string, error) {
	allPkgs, err := LoadAllPackageData(ctx)
	if err != nil {
		return nil, err
	}
	rv := []string{}
	for name, pkg := range allPkgs {
		if util.In(findPkg, pkg.Imports) {
			rv = append(rv, name)
		}
	}
	return rv, nil
}
