package cipd

/*
	Utilities for working with CIPD.
*/

import (
	"context"
	"fmt"

	"github.com/luci/luci-go/cipd/client/cipd"
	"github.com/luci/luci-go/cipd/client/cipd/common"
	"go.skia.org/infra/go/auth"
)

const (
	SERVICE_URL = "https://chrome-infra-packages.appspot.com"

	PKG_VERSION_GO = 2
)

var (
	PkgGo = &Package{
		Dest:    "go",
		Name:    "skia/bots/go",
		Version: fmt.Sprintf("version:%d", PKG_VERSION_GO),
	}
)

type Package struct {
	Dest    string
	Name    string
	Version string
}

// Run "cipd ensure" to get the correct packages in the given location.
func Ensure(local bool, rootdir string, packages ...*Package) error {
	httpClient, err := auth.NewDefaultClient(local, auth.SCOPE_USERINFO_EMAIL, auth.SCOPE_PLUS_ME)
	if err != nil {
		return fmt.Errorf("Failed to create authenticated HTTP client: %s", err)
	}
	c, err := cipd.NewClient(cipd.ClientOptions{
		ServiceURL:          SERVICE_URL,
		Root:                rootdir,
		AuthenticatedClient: httpClient,
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
		pkgs[pkg.Dest] = common.PinSlice{pin}
	}
	if _, err := c.EnsurePackages(context.Background(), pkgs, false); err != nil {
		return fmt.Errorf("Failed to ensure packages: %s", err)
	}
	return nil
}
