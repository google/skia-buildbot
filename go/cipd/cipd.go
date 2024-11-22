package cipd

/*
	Utilities for working with CIPD.
*/

//go:generate bazelisk run --config=mayberemote //:go -- run gen_versions.go

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"

	api "go.chromium.org/luci/cipd/api/cipd/v1"
	"go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/client/cipd/builder"
	"go.chromium.org/luci/cipd/client/cipd/ensure"
	"go.chromium.org/luci/cipd/client/cipd/fs"
	"go.chromium.org/luci/cipd/client/cipd/pkg"
	"go.chromium.org/luci/cipd/client/cipd/template"
	"go.chromium.org/luci/cipd/common"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	// CIPD server to use for obtaining packages.
	DefaultServiceURL = "https://chrome-infra-packages.appspot.com"

	// Platforms supported by CIPD.
	PlatformLinuxAmd64   = "linux-amd64"
	PlatformLinuxArm64   = "linux-arm64"
	PlatformLinuxArmv6l  = "linux-armv6l"
	PlatformMacAmd64     = "mac-amd64"
	PlatformWindows386   = "windows-386"
	PlatformWindowsAmd64 = "windows-amd64"

	// Placeholder for target platform.
	PlatformPlaceholder = "${platform}"

	// This is the CIPD package containing CIPD itself.
	PkgNameCIPD = "infra/tools/cipd/${os}-${arch}"

	// Template for Git CIPD package for a particular platform.
	pkgGitTmpl = "infra/3pp/tools/git/%s"

	// Template for cpython CIPD package for a particular platform.
	pkgCpythonTmpl  = "infra/3pp/tools/cpython/%s"
	pkgCpython3Tmpl = "infra/3pp/tools/cpython3/%s"
)

var (
	// Platforms are the known CIPD platforms.
	Platforms = []string{
		PlatformLinuxAmd64,
		PlatformLinuxArm64,
		PlatformLinuxArmv6l,
		PlatformMacAmd64,
		PlatformWindows386,
		PlatformWindowsAmd64,
	}

	// CIPD package for CIPD itself.
	PkgCIPD = MustGetPackage(PkgNameCIPD)

	// CIPD package for the Go installation.
	PkgGo = MustGetPackage("skia/bots/go")

	// CIPD package containing the Google Protocol Buffer compiler.
	PkgProtoc = MustGetPackage("skia/bots/protoc")

	// CIPD packages required for using Git.
	PkgsGit = map[string][]*Package{
		PlatformLinuxAmd64: {
			MustGetPackage(fmt.Sprintf(pkgGitTmpl, PlatformLinuxAmd64)),
			MustGetPackage("infra/tools/git/${platform}"),
			MustGetPackage("infra/tools/luci/git-credential-luci/${platform}"),
		},
		PlatformLinuxArm64: {
			MustGetPackage(fmt.Sprintf(pkgGitTmpl, PlatformLinuxArm64)),
			MustGetPackage("infra/tools/git/${platform}"),
			MustGetPackage("infra/tools/luci/git-credential-luci/${platform}"),
		},
		PlatformLinuxArmv6l: {
			MustGetPackage(fmt.Sprintf(pkgGitTmpl, PlatformLinuxArmv6l)),
			MustGetPackage("infra/tools/git/${platform}"),
			MustGetPackage("infra/tools/luci/git-credential-luci/${platform}"),
		},
		PlatformMacAmd64: {
			MustGetPackage(fmt.Sprintf(pkgGitTmpl, PlatformMacAmd64)),
			MustGetPackage("infra/tools/git/${platform}"),
			MustGetPackage("infra/tools/luci/git-credential-luci/${platform}"),
		},
		PlatformWindows386: {
			MustGetPackage(fmt.Sprintf(pkgGitTmpl, PlatformWindows386)),
			MustGetPackage("infra/tools/git/${platform}"),
			MustGetPackage("infra/tools/luci/git-credential-luci/${platform}"),
		},
		PlatformWindowsAmd64: {
			MustGetPackage(fmt.Sprintf(pkgGitTmpl, PlatformWindowsAmd64)),
			MustGetPackage("infra/tools/git/${platform}"),
			MustGetPackage("infra/tools/luci/git-credential-luci/${platform}"),
		},
	}

	// CIPD packages required for using Python.
	PkgsPython = map[string][]*Package{
		PlatformLinuxAmd64: {
			MustGetPackage(fmt.Sprintf(pkgCpython3Tmpl, PlatformLinuxAmd64)),
			MustGetPackage("infra/tools/luci/vpython3/${platform}"),
		},
		PlatformLinuxArm64: {
			MustGetPackage(fmt.Sprintf(pkgCpython3Tmpl, PlatformLinuxArm64)),
			MustGetPackage("infra/tools/luci/vpython3/${platform}"),
		},
		PlatformLinuxArmv6l: {
			MustGetPackage(fmt.Sprintf(pkgCpython3Tmpl, PlatformLinuxArmv6l)),
			MustGetPackage("infra/tools/luci/vpython3/${platform}"),
		},
		PlatformMacAmd64: {
			MustGetPackage(fmt.Sprintf(pkgCpython3Tmpl, PlatformMacAmd64)),
			MustGetPackage("infra/tools/luci/vpython3/${platform}"),
		},
		PlatformWindows386: {
			MustGetPackage(fmt.Sprintf(pkgCpython3Tmpl, PlatformWindows386)),
			MustGetPackage("infra/tools/luci/vpython3/${platform}"),
		},
		PlatformWindowsAmd64: {
			MustGetPackage(fmt.Sprintf(pkgCpython3Tmpl, PlatformWindowsAmd64)),
			MustGetPackage("infra/tools/luci/vpython3/${platform}"),
		},
	}
)

// VersionTag returns a CIPD version tag for the given version number.
func VersionTag(version string) string {
	return fmt.Sprintf("version:%s", version)
}

// Package describes a CIPD package.
type Package struct {
	// Name of the package.
	Name string `json:"name"`

	// Relative path within the root dir to install the package.
	Path string `json:"path"`

	// Version of the package. See the CIPD docs for valid version strings:
	// https://godoc.org/go.chromium.org/luci/cipd/common#ValidateInstanceVersion
	Version string `json:"version"`
}

func (p *Package) String() string {
	return fmt.Sprintf("%s:%s:%s", p.Path, p.Name, p.Version)
}

// PackageSlice is used for sorting packages by name.
type PackageSlice []*Package

func (s PackageSlice) Len() int           { return len(s) }
func (s PackageSlice) Less(i, j int) bool { return s[i].Name < s[j].Name }
func (s PackageSlice) Swap(i, j int)      { s[i], s[j] = s[j], s[i] }

// GetPackage returns the definition for the package with the given name, or an
// error if the package does not exist in the registry.
func GetPackage(pkg string) (*Package, error) {
	rv, ok := PACKAGES[pkg]
	if !ok {
		return nil, skerr.Fmt("Unknown CIPD package %q", pkg)
	}
	return rv, nil
}

// MustGetPackage returns the definition for the package with the given name.
// Panics if the package does not exist in the registry.
func MustGetPackage(pkg string) *Package {
	rv, err := GetPackage(pkg)
	if err != nil {
		sklog.Fatal(err)
	}
	return rv
}

// Utility function that returns CIPD packages as slice of strings. Created for
// go/swarming, this can be removed when go/swarming has no more clients.
func GetStrCIPDPkgs(pkgs []*Package) []string {
	cipdPkgs := []string{}
	for _, p := range pkgs {
		cipdPkgs = append(cipdPkgs, p.String())
	}
	return cipdPkgs
}

// Run "cipd ensure" to get the correct packages in the given location. Note
// that any previously-installed packages in the given rootDir will be removed
// if not specified again.
func Ensure(ctx context.Context, c *http.Client, rootDir string, forceCopyInstallMode bool, packages ...*Package) error {
	cipdClient, err := NewClient(c, rootDir, DefaultServiceURL)
	if err != nil {
		return skerr.Wrapf(err, "failed to create CIPD client")
	}
	return cipdClient.Ensure(ctx, forceCopyInstallMode, packages...)
}

// ParseEnsureFile parses a CIPD ensure file and returns a slice of Packages.
func ParseEnsureFile(file string) ([]*Package, error) {
	var ensureFile *ensure.File
	if err := util.WithReadFile(file, func(r io.Reader) error {
		f, err := ensure.ParseFile(r)
		if err == nil {
			ensureFile = f
		}
		return err
	}); err != nil {
		return nil, skerr.Wrapf(err, "Failed to parse CIPD ensure file %s", file)
	}
	var rv []*Package
	for subdir, pkgSlice := range ensureFile.PackagesBySubdir {
		if subdir == "" {
			subdir = "."
		}
		for _, pkg := range pkgSlice {
			rv = append(rv, &Package{
				Path:    subdir,
				Name:    pkg.PackageTemplate,
				Version: pkg.UnresolvedVersion,
			})
		}
	}
	return rv, nil
}

// CIPDClient is the interface for interactions with the CIPD API.
type CIPDClient interface {
	cipd.Client

	// Attach the given refs, tags, and metadata to the given package instance.
	Attach(ctx context.Context, pin common.Pin, refs []string, tags []string, metadata map[string]string) error

	// Create uploads a new package instance.
	Create(ctx context.Context, name, dir string, installMode pkg.InstallMode, excludeMatchingFiles []*regexp.Regexp, refs []string, tags []string, metadata map[string]string) (common.Pin, error)

	// Ensure runs "cipd ensure" to get the correct packages in the given location. Note
	// that any previously-installed packages in the given rootDir will be removed
	// if not specified again.
	Ensure(ctx context.Context, forceCopyInstallMode bool, packages ...*Package) error

	// Describe is a convenience wrapper around cipd.Client.DescribeInstance.
	Describe(ctx context.Context, pkg, instance string, includeMetadata bool) (*cipd.InstanceDescription, error)
}

// Client is a struct used for interacting with the CIPD API.
type Client struct {
	cipd.Client
	expander template.Expander
}

// NewClient returns a CIPD client.
func NewClient(c *http.Client, rootDir, serviceURL string) (*Client, error) {
	cipdClient, err := cipd.NewClient(cipd.ClientOptions{
		ServiceURL:          serviceURL,
		Root:                rootDir,
		AuthenticatedClient: c,
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "failed to create CIPD client")
	}
	return &Client{
		Client:   cipdClient,
		expander: template.DefaultExpander(),
	}, nil
}

func (c *Client) Ensure(ctx context.Context, forceCopyInstallMode bool, packages ...*Package) error {
	pkgs := common.PinSliceBySubdir{}
	for _, pkg := range packages {
		pkgName, err := c.expander.Expand(pkg.Name)
		if err != nil {
			return skerr.Wrapf(err, "failed to expand package name %q", pkg.Name)
		}
		pin, err := c.ResolveVersion(ctx, pkgName, pkg.Version)
		if err != nil {
			return skerr.Wrapf(err, "failed to resolve package version %q @ %q", pkgName, pkg.Version)
		}
		sklog.Infof("Installing version %s (from %s) of %s", pin.InstanceID, pkg.Version, pin.PackageName)
		pkgs[pkg.Path] = append(pkgs[pkg.Path], pin)
	}
	opts := &cipd.EnsureOptions{
		Paranoia: cipd.CheckPresence,
		DryRun:   false,
	}
	if forceCopyInstallMode {
		opts.OverrideInstallMode = pkg.InstallModeCopy
	}
	if _, err := c.EnsurePackages(ctx, pkgs, opts); err != nil {
		return skerr.Wrapf(err, "failed to ensure packages")
	}
	return nil
}

func (c *Client) Describe(ctx context.Context, pkg, instance string, includeMetadata bool) (*cipd.InstanceDescription, error) {
	pin := common.Pin{
		PackageName: pkg,
		InstanceID:  instance,
	}
	opts := &cipd.DescribeInstanceOpts{
		DescribeRefs:     true,
		DescribeTags:     true,
		DescribeMetadata: includeMetadata,
	}
	return c.DescribeInstance(ctx, pin, opts)
}

func (c *Client) Create(ctx context.Context, name, dir string, installMode pkg.InstallMode, excludeMatchingFiles []*regexp.Regexp, refs []string, tags []string, metadata map[string]string) (rv common.Pin, rvErr error) {
	// Find the files to be included in the package.
	filter := func(path string) bool {
		for _, regex := range excludeMatchingFiles {
			if regex.MatchString(path) {
				return true
			}
		}
		return false
	}
	files, err := fs.ScanFileSystem(dir, dir, filter, fs.ScanOptions{
		PreserveModTime:  false,
		PreserveWritable: false,
	})
	if err != nil {
		return common.Pin{}, skerr.Wrap(err)
	}

	// Create the package file.
	tmp, err := os.MkdirTemp("", "")
	if err != nil {
		return common.Pin{}, skerr.Wrap(err)
	}
	defer func() {
		if err := os.RemoveAll(tmp); err != nil {
			if rvErr != nil {
				rvErr = skerr.Wrap(err)
			}
		}
	}()
	pkgFile := filepath.Join(tmp, "cipd.pkg")
	f, err := os.Create(pkgFile)
	if err != nil {
		return common.Pin{}, skerr.Wrap(err)
	}

	// Build the package.
	buildOpts := builder.Options{
		CompressionLevel: 1,
		Input:            files,
		InstallMode:      installMode,
		Output:           f,
		PackageName:      name,
	}
	pin, err := builder.BuildInstance(ctx, buildOpts)
	if err != nil {
		_ = f.Close()
		return common.Pin{}, skerr.Wrap(err)
	}
	if err := f.Close(); err != nil {
		return common.Pin{}, skerr.Wrap(err)
	}

	// Register the instance.
	src, err := pkg.NewFileSource(pkgFile)
	if err != nil {
		return common.Pin{}, skerr.Wrap(err)
	}
	if err := c.RegisterInstance(ctx, pin, src, cipd.CASFinalizationTimeout); err != nil {
		return common.Pin{}, skerr.Wrap(err)
	}

	// Apply the given refs and tags.
	if err := c.Attach(ctx, pin, refs, tags, metadata); err != nil {
		return common.Pin{}, skerr.Wrap(err)
	}
	return pin, nil
}

func (c *Client) Attach(ctx context.Context, pin common.Pin, refs []string, tags []string, metadata map[string]string) error {
	if len(metadata) > 0 {
		md := make([]cipd.Metadata, 0, len(metadata))
		for k, v := range metadata {
			md = append(md, cipd.Metadata{
				Key:   k,
				Value: []byte(v),
			})
		}
		if err := c.AttachMetadataWhenReady(ctx, pin, md); err != nil {
			return skerr.Wrap(err)
		}
	}
	if len(tags) > 0 {
		if err := c.AttachTagsWhenReady(ctx, pin, tags); err != nil {
			return skerr.Wrap(err)
		}
	}
	for _, ref := range refs {
		if err := c.SetRefWhenReady(ctx, ref, pin); err != nil {
			return skerr.Wrap(err)
		}
	}
	return nil
}

// Ensure that Client implements CIPDClient.
var _ CIPDClient = &Client{}

// Sha256ToInstanceID returns a package instance ID based on a sha256 sum.
func Sha256ToInstanceID(sha256 string) (string, error) {
	ref := &api.ObjectRef{
		HashAlgo:  api.HashAlgo_SHA256,
		HexDigest: sha256,
	}
	if err := common.ValidateObjectRef(ref, common.KnownHash); err != nil {
		return "", skerr.Wrap(err)
	}
	return common.ObjectRefToInstanceID(ref), nil
}

// InstanceIDToSha256 returns a sha256 based on a package instance ID.
func InstanceIDToSha256(instanceID string) (string, error) {
	if err := common.ValidateInstanceID(instanceID, common.KnownHash); err != nil {
		return "", skerr.Wrap(err)
	}
	ref := common.InstanceIDToObjectRef(instanceID)
	if ref.HashAlgo != api.HashAlgo_SHA256 {
		return "", skerr.Fmt("instance ID %q does not use sha256 (uses %s)", instanceID, ref.HashAlgo.String())
	}
	return ref.HexDigest, nil
}
