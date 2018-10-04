package go_install

/*
	This package aids in obtaining an installation of Go from CIPD.
*/

import (
	"fmt"
	"net/http"
	"os"
	"path"

	"go.skia.org/infra/go/cipd"
)

// EnsureGo ensures that the Go installation is obtained from CIPD at the
// correct version and returns the path to the go binary and the relevant
// environment variables or any errors which occurred. If includeDeps is true,
// also installs the go_deps CIPD package which contains all dependencies for
// the go.skia.org/infra repository as of the last update of that package.
func EnsureGo(client *http.Client, cipdRoot string, includeDeps bool) (string, map[string]string, error) {
	pkgs := []*cipd.Package{cipd.PkgGo}
	if includeDeps {
		pkgs = append(pkgs, cipd.PkgGoDEPS)
	}
	if err := cipd.Ensure(client, cipdRoot, pkgs...); err != nil {
		return "", nil, fmt.Errorf("Failed to ensure Go CIPD package: %s", err)
	}
	// Set the GOPATH to be the destination of the go_deps CIPD package.
	// If we aren't installing that package, we need to create the dir so
	// that "go get" works properly.
	goPath := path.Join(cipdRoot, cipd.PkgGoDEPS.Dest)
	if !includeDeps {
		if err := os.MkdirAll(goPath, os.ModePerm); err != nil {
			return "", nil, fmt.Errorf("Failed to create GOPATH: %s", err)
		}
	}
	goRoot := path.Join(cipdRoot, cipd.PkgGo.Dest, "go")
	goBin := path.Join(goRoot, "bin")
	return path.Join(goBin, "go"), map[string]string{
		"GOPATH": goPath,
		"GOROOT": goRoot,
		"PATH":   fmt.Sprintf("%s:%s", goBin, path.Join(goPath, "bin")),
	}, nil
}
