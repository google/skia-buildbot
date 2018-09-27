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
// environment variables or any errors which occurred.
func EnsureGo(client *http.Client, workdir string) (string, map[string]string, error) {
	root := path.Join(workdir, "cipd")
	if err := cipd.Ensure(client, root, cipd.PkgGo); err != nil {
		return "", nil, fmt.Errorf("Failed to ensure Go CIPD package: %s", err)
	}
	goPath := path.Join(workdir, "gopath")
	if err := os.MkdirAll(goPath, os.ModePerm); err != nil {
		return "", nil, fmt.Errorf("Failed to create GOPATH: %s", err)
	}
	goRoot := path.Join(root, "go", "go")
	goBin := path.Join(goRoot, "bin")
	return path.Join(goBin, "go"), map[string]string{
		"GOPATH": goPath,
		"GOROOT": goRoot,
		"PATH":   fmt.Sprintf("%s:%s", goBin, path.Join(goPath, "bin")),
	}, nil
}
