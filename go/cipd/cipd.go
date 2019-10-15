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
// that any previously-installed packages in the given rootDir will be removed
// if not specified again.
func Ensure(ctx context.Context, c *http.Client, rootDir string, packages ...*Package) error {
	cipdClient, err := NewClient(c, rootDir)
	if err != nil {
		return fmt.Errorf("Failed to create CIPD client: %s", err)
	}
	return cipdClient.Ensure(ctx, packages...)
}

// CIPDClient is the interface for interactions with the CIPD API.
type CIPDClient interface {
	cipd.Client

	// Ensure runs "cipd ensure" to get the correct packages in the given location. Note
	// that any previously-installed packages in the given rootDir will be removed
	// if not specified again.
	Ensure(ctx context.Context, packages ...*Package) error

	// Describe is a convenience wrapper around cipd.Client.DescribeInstance.
	Describe(ctx context.Context, pkg, instance string) (*cipd.InstanceDescription, error)
}

// Client is a struct used for interacting with the CIPD API.
type Client struct {
	cipd.Client
}

// NewClient returns a CIPD client.
func NewClient(c *http.Client, rootDir string) (*Client, error) {
	cipdClient, err := cipd.NewClient(cipd.ClientOptions{
		ServiceURL:          SERVICE_URL,
		Root:                rootDir,
		AuthenticatedClient: c,
	})
	if err != nil {
		return nil, fmt.Errorf("Failed to create CIPD client: %s", err)
	}
	return &Client{cipdClient}, nil
}

func (c *Client) Ensure(ctx context.Context, packages ...*Package) error {
	pkgs := common.PinSliceBySubdir{}
	for _, pkg := range packages {
		pin, err := c.ResolveVersion(ctx, pkg.Name, pkg.Version)
		if err != nil {
			return fmt.Errorf("Failed to resolve package version %q @ %q: %s", pkg.Name, pkg.Version, err)
		}
		sklog.Infof("Installing version %s (from %s) of %s", pin.InstanceID, pkg.Version, pkg.Name)
		pkgs[pkg.Dest] = common.PinSlice{pin}
	}
	// This means use as many threads as CPUs. (Prior to
	// https://chromium-review.googlesource.com/c/infra/luci/luci-go/+/1848212,
	// extracting the packages was always single-threaded.)
	const maxThreads = 0
	if _, err := c.EnsurePackages(ctx, pkgs, cipd.CheckPresence, maxThreads, false); err != nil {
		return fmt.Errorf("Failed to ensure packages: %s", err)
	}
	return nil
}

func (c *Client) Describe(ctx context.Context, pkg, instance string) (*cipd.InstanceDescription, error) {
	pin := common.Pin{
		PackageName: pkg,
		InstanceID:  instance,
	}
	opts := &cipd.DescribeInstanceOpts{
		DescribeRefs: true,
		DescribeTags: true,
	}
	return c.DescribeInstance(ctx, pin, opts)
}
