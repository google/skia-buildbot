package go_install

/*
	This package aids in obtaining an installation of Go from CIPD.
*/

import (
	"context"
	"fmt"
	"net/http"
	"path"

	"go.skia.org/infra/go/cipd"
)

// EnsureGo ensures that the Go installation is obtained from CIPD at the
// correct version and returns the path to the go binary and the relevant
// environment variables or any errors which occurred. If includeDeps is true,
// also installs the go_deps CIPD package which contains all dependencies for
// the go.skia.org/infra repository as of the last update of that package.
func EnsureGo(ctx context.Context, client *http.Client, cipdRoot string) (string, map[string]string, error) {
	pkgs := []*cipd.Package{cipd.PkgGo}
	if err := cipd.Ensure(ctx, client, cipdRoot, pkgs...); err != nil {
		return "", nil, fmt.Errorf("Failed to ensure Go CIPD package: %s", err)
	}
	goRoot := path.Join(cipdRoot, cipd.PkgGo.Dest, "go")
	goBin := path.Join(goRoot, "bin")
	return path.Join(goBin, "go"), map[string]string{
		"GO111MODULE": "on",
		"GOROOT":      goRoot,
		"PATH":        goBin,
	}, nil
}
