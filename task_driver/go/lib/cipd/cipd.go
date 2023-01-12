package cipd

import (
	"context"
	"flag"
	"net/http"
	"strings"

	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/task_driver/go/td"
)

type Flags *[]string

// SetupFlags initializes command-line flags used by this package. If a FlagSet
// is not provided, then these become top-level CommandLine flags.
func SetupFlags(fs *flag.FlagSet) Flags {
	if fs == nil {
		fs = flag.CommandLine
	}
	return common.FSNewMultiStringFlag(fs, "cipd", nil, "CIPD packages to install, in the form: \"dest/dir:package/name@version\"")
}

// Ensure installs the given CIPD packages.
func Ensure(ctx context.Context, c *http.Client, workdir string, pkgs ...*cipd.Package) error {
	return td.Do(ctx, td.Props("Download CIPD Packages").Infra(), func(ctx context.Context) error {
		if len(pkgs) > 0 {
			return cipd.Ensure(ctx, c, workdir, pkgs...)
		}
		return nil
	})
}

// EnsureFromFlags installs the CIPD packages requested using the given flags.
func EnsureFromFlags(ctx context.Context, c *http.Client, workdir string, f Flags) error {
	return td.Do(ctx, td.Props("Download CIPD Packages").Infra(), func(ctx context.Context) error {
		pkgs, err := GetPackages(f)
		if err != nil {
			return skerr.Wrap(err)
		}
		if len(pkgs) > 0 {
			return cipd.Ensure(ctx, c, workdir, pkgs...)
		}
		return nil
	})
}

// GetPackages creates a slice of cipd.Package from the Flags.
func GetPackages(f Flags) ([]*cipd.Package, error) {
	if len(*f) == 0 {
		return nil, nil
	}
	rv := make([]*cipd.Package, 0, len(*f))
	for _, flagStr := range *f {
		pkg := &cipd.Package{}
		pathSplit := strings.SplitN(flagStr, ":", 2)
		if len(pathSplit) != 2 {
			return nil, skerr.Fmt("Expected flag in the form \"dest/dir:package/name@version\" but got %q", flagStr)
		}
		pkg.Path = pathSplit[0]
		versionSplit := strings.SplitN(pathSplit[1], "@", 2)
		if len(versionSplit) != 2 {
			return nil, skerr.Fmt("Expected flag in the form \"dest/dir:package/name@version\" but got %q", flagStr)
		}
		pkg.Name = versionSplit[0]
		pkg.Version = versionSplit[1]
		rv = append(rv, pkg)
	}
	return rv, nil
}
