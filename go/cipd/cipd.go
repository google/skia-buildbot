package cipd

/*
	Utilities for working with CIPD.
*/

//go:generate go run gen_versions.go

import (
	"context"
	"fmt"
	"net/http"

	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/common"
	"go.skia.org/infra/go/sklog"
)

const (
	// CIPD server to use for obtaining packages.
	SERVICE_URL = "https://chrome-infra-packages.appspot.com"
)

var (
	// CIPD package for the Go installation.
	PkgGo = &Package{
		Dest:    "go",
		Name:    "skia/bots/go",
		Version: VersionTag(PKG_VERSIONS_FROM_ASSETS["go"]),
	}

	// CIPD package containing the Google Protocol Buffer compiler.
	PkgProtoc = &Package{
		Dest:    "protoc",
		Name:    "skia/bots/protoc",
		Version: VersionTag(PKG_VERSIONS_FROM_ASSETS["protoc"]),
	}
)

// VersionTag returns a CIPD version tag for the given version number.
func VersionTag(version string) string {
	return fmt.Sprintf("version:%s", version)
}

// Package describes a CIPD package.
type Package struct {
	// Relative path within the root dir to install the package.
	Dest string

	// Name of the package.
	Name string

	// Version of the package. See the CIPD docs for valid version strings:
	// https://godoc.org/go.chromium.org/luci/cipd/common#ValidateInstanceVersion
	Version string
}

// Run "cipd ensure" to get the correct packages in the given location. Note
// that any previously-installed packages in the given rootdir will be removed
// if not specified again.
func Ensure(client *http.Client, rootdir string, packages ...*Package) error {
	c, err := cipd.NewClient(cipd.ClientOptions{
		ServiceURL:          SERVICE_URL,
		Root:                rootdir,
		AuthenticatedClient: client,
	})
	if err != nil {
		return fmt.Errorf("Failed to create CIPD client: %s", err)
	}

	ctx := context.Background()
	pkgs := common.PinSliceBySubdir{}
	for _, pkg := range packages {
		pin, err := c.ResolveVersion(ctx, pkg.Name, pkg.Version)
		if err != nil {
			return fmt.Errorf("Failed to resolve package version %q @ %q: %s", pkg.Name, pkg.Version, err)
		}
		sklog.Infof("Installing version %s (from %s) of %s", pin.InstanceID, pkg.Version, pkg.Name)
		pkgs[pkg.Dest] = common.PinSlice{pin}
	}
	if _, err := c.EnsurePackages(context.Background(), pkgs, cipd.CheckPresence, false); err != nil {
		return fmt.Errorf("Failed to ensure packages: %s", err)
	}
	return nil
}
