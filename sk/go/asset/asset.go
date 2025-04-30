package asset

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	os_exec "os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
	cipd_api "go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/client/cipd/pkg"
	"go.chromium.org/luci/cipd/common"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/httputils/progress"
	"go.skia.org/infra/go/luciauth"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

const (
	cipdPackageNameTmpl = "skia/bots/%s"

	tagProject       = "project:skia"
	tagVersionPrefix = "version:"
	tagVersionTmpl   = tagVersionPrefix + "%d"

	creationScriptBaseName        = "create.py"
	creationScriptInitialContents = `#!/usr/bin/env python3
#
# Copyright 2017 Google Inc.
#
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Create the asset."""


import argparse


def create_asset(target_dir):
  """Create the asset."""
  raise NotImplementedError('Implement me!')


def main():
  parser = argparse.ArgumentParser()
  parser.add_argument('--target_dir', '-t', required=True)
  args = parser.parse_args()
  create_asset(args.target_dir)


if __name__ == '__main__':
  main()

`

	assetSettingsFileBaseName = "asset.json"
	versionFileBaseName       = "VERSION"
)

var (
	skipFilesRegex = []*regexp.Regexp{
		regexp.MustCompile(`^\.git$`),
		regexp.MustCompile(`^\.svn$`),
		regexp.MustCompile(`.*\.pyc$`),
		regexp.MustCompile(`^.DS_STORE$`),
	}
	tagVersionRegex = regexp.MustCompile(tagVersionPrefix + `(\d+)`)
)

// Command returns a cli.Command instance which represents the "asset" command.
func Command() *cli.Command {
	flagIn := "in"
	flagDryRun := "dry-run"
	flagTags := "tags"
	flagCI := "ci"
	return &cli.Command{
		Name:        "asset",
		Description: "Manage assets used by developers and CI.",
		Usage:       "asset <subcommand>",
		Subcommands: []*cli.Command{
			{
				Name:        "add",
				Description: "Add a new asset.",
				Usage:       "add <asset name>",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 1 {
						return skerr.Fmt("Expected exactly one positional argument.")
					}
					return cmdAdd(ctx.Context, args[0])
				},
			},
			{
				Name:        "remove",
				Description: "Remove an existing asset. Does not delete uploaded packages.",
				Usage:       "remove <asset name>",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 1 {
						return skerr.Fmt("Expected exactly one positional argument.")
					}
					return cmdRemove(ctx.Context, args[0])
				},
			},
			{
				Name:        "download",
				Description: "Download an asset.",
				Usage:       "download <asset name> <target directory>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  flagCI,
						Value: false,
						Usage: "Indicates that we're running in CI, as opposed to a developer machine.",
					},
				},
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 2 {
						return skerr.Fmt("Expected exactly two positional arguments.")
					}
					return cmdDownload(ctx.Context, args[0], args[1], !ctx.Bool(flagCI))
				},
			},
			{
				Name:        "upload",
				Description: "Upload a new version of the asset. If --in is provided, use the contents of the provided directory, otherwise run the creation script to generate the package contents.",
				Usage:       "upload <asset name>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  flagIn,
						Value: "",
						Usage: "Use the contents of this directory as the package contents. If not provided, expects a creation script to be present within the asset dir.",
					},
					&cli.BoolFlag{
						Name:  flagDryRun,
						Value: false,
						Usage: "Create the package, including running any automation scripts, but do not upload it.",
					},
					&cli.StringSliceFlag{
						Name:  flagTags,
						Usage: "Any additional tags to apply to the package, in \"key:value\" format. May be specified multiple times.",
					},
					&cli.BoolFlag{
						Name:  flagCI,
						Value: false,
						Usage: "Indicates that we're running in CI, as opposed to a developer machine.",
					},
				},
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 1 {
						return skerr.Fmt("Expected exactly one positional argument.")
					}
					return cmdUpload(ctx.Context, args[0], ctx.String(flagIn), ctx.Bool(flagDryRun), ctx.StringSlice(flagTags), !ctx.Bool(flagCI))
				},
			},
			{
				Name:        "get-version",
				Description: "Print the current version of the asset.",
				Usage:       "get-version <asset name>",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 1 {
						return skerr.Fmt("Expected exactly one positional argument.")
					}
					ver, err := getVersion(args[0])
					if err != nil {
						return skerr.Wrap(err)
					}
					fmt.Println(strconv.Itoa(ver))
					return nil
				},
			},
			{
				Name:        "set-version",
				Description: "Manually set the asset to an already-uploaded version.",
				Usage:       "set-version <asset name> <version>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  flagCI,
						Value: false,
						Usage: "Indicates that we're running in CI, as opposed to a developer machine.",
					},
				},
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 2 {
						return skerr.Fmt("Expected exactly two positional arguments.")
					}
					version, err := strconv.Atoi(args[1])
					if err != nil {
						return skerr.Wrapf(err, "invalid version number")
					}
					return cmdSetVersion(ctx.Context, args[0], version, !ctx.Bool(flagCI))
				},
			},
			{
				Name:        "list-versions",
				Description: "List the uploaded versions of the asset.",
				Usage:       "list-versions <asset name>",
				Flags: []cli.Flag{
					&cli.BoolFlag{
						Name:  flagCI,
						Value: false,
						Usage: "Indicates that we're running in CI, as opposed to a developer machine.",
					},
				},
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 1 {
						return skerr.Fmt("Expected exactly one positional argument.")
					}
					return cmdListVersions(ctx.Context, args[0], !ctx.Bool(flagCI))
				},
			},
		},
	}
}

// cmdAdd implements the "add" subcommand.
func cmdAdd(ctx context.Context, name string) error {
	// Create the asset directory.
	assetDir, err := getAssetDir(name)
	if err != nil {
		return skerr.Wrap(err)
	}
	if _, err := os.Stat(assetDir); !os.IsNotExist(err) {
		return skerr.Fmt("Asset %q already exists in %s", name, assetDir)
	}
	if err := os.MkdirAll(assetDir, os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}

	// Write the initial (empty) version file.
	versionFile := filepath.Join(assetDir, versionFileBaseName)
	if err := os.WriteFile(versionFile, []byte{}, os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}

	// Optionally write a creation script skeleton.
	fmt.Printf("Do you want to add a creation script for this asset? (y/n): ")
	read, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return skerr.Wrap(err)
	}
	read = strings.TrimSpace(read)
	if read == "y" {
		creationScript := filepath.Join(assetDir, creationScriptBaseName)
		if err := os.WriteFile(creationScript, []byte(creationScriptInitialContents), os.ModePerm); err != nil {
			return skerr.Wrap(err)
		}
		fmt.Printf("Created %s; you will need to add implementation before uploading the asset.\n", creationScript)
	}

	// "git add" the new directory.
	if _, err := git.CheckoutDir(".").Git(ctx, "add", assetDir); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// cmdRemove implements the "remove" subcommand.
func cmdRemove(ctx context.Context, name string) error {
	assetDir, err := getAssetDir(name)
	if err != nil {
		return skerr.Wrap(err)
	}
	if _, err := git.CheckoutDir(".").Git(ctx, "rm", "-rf", assetDir); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// getCIPDClient creates and returns a cipd.CIPDClient.
func getCIPDClient(ctx context.Context, rootDir string, local bool) (cipd.CIPDClient, *progress.ProgressTracker, *progress.ProgressTracker, error) {
	var ts oauth2.TokenSource
	var err error
	if local {
		ts, err = google.DefaultTokenSource(ctx, auth.ScopeUserinfoEmail)
	} else {
		ts, err = luciauth.NewLUCIContextTokenSource(auth.ScopeUserinfoEmail)
	}
	if err != nil {
		return nil, nil, nil, skerr.Wrapf(err, "Could not get token source. \nTry running 'gcloud auth application-default login'\n")
	}
	authenticatedClient := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	anonymousClient, uploadTracker, downloadTracker := progress.ProgressTrackingClient(http.DefaultClient)
	opts := cipd_api.ClientOptions{
		ServiceURL:          cipd.DefaultServiceURL,
		Root:                rootDir,
		AuthenticatedClient: authenticatedClient,
		// The CIPD client uses the provided AnonymousClient to perform the
		// actual uploads/downloads, not the AuthenticatedClient. Reusing the
		// authenticated http.Client results in a 403.
		AnonymousClient: anonymousClient,
	}
	cipdClientInner, err := cipd_api.NewClient(opts)
	if err != nil {
		return nil, nil, nil, skerr.Wrap(err)
	}
	return &cipd.Client{Client: cipdClientInner}, uploadTracker, downloadTracker, nil
}

// cmdDownload implements the "download" subcommand.
func cmdDownload(ctx context.Context, name, dest string, local bool) error {
	cipdClient, _, downloadTracker, err := getCIPDClient(ctx, dest, local)
	if err != nil {
		return skerr.Wrap(err)
	}
	version, err := getVersion(name)
	if err != nil {
		return skerr.Wrap(err)
	}
	versionTag := fmt.Sprintf(tagVersionTmpl, version)
	packagePath := fmt.Sprintf(cipdPackageNameTmpl, name)
	pin, err := cipdClient.ResolveVersion(ctx, packagePath, versionTag)
	if err != nil {
		return skerr.Wrap(err)
	}
	fmt.Printf("Downloading %s\n", pin.String())
	downloadTracker.Start()
	defer downloadTracker.Stop()
	if _, err := cipdClient.EnsurePackages(ctx, common.PinSliceBySubdir{
		"": []common.Pin{pin},
	}, &cipd_api.EnsureOptions{
		Paranoia:            cipd_api.CheckPresence,
		DryRun:              false,
		OverrideInstallMode: pkg.InstallModeCopy,
	}); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// cmdUpload implements the "upload" subcommand.
func cmdUpload(ctx context.Context, name, src string, dryRun bool, extraTags []string, local bool) (rvErr error) {
	// Validate extraTags.
	for _, tag := range extraTags {
		if len(strings.Split(tag, ":")) != 2 {
			return skerr.Fmt("Tags must be in the form \"key:value\", not %q", tag)
		}
	}

	// Read the asset settings.
	settings, err := readAssetSettings(name)
	if err != nil {
		return skerr.Wrap(err)
	}

	cipdClient, uploadTracker, _, err := getCIPDClient(ctx, ".", local)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Run the creation script, if one exists.
	assetDir, err := getAssetDir(name)
	if err != nil {
		return skerr.Wrap(err)
	}
	creationScript := filepath.Join(assetDir, creationScriptBaseName)
	if _, err := os.Stat(creationScript); err == nil {
		if src != "" {
			return skerr.Fmt("Target directory is not supplied when using a creation script.")
		}
		src, err = os.MkdirTemp("", "")
		if err != nil {
			return skerr.Wrap(err)
		}
		defer func() {
			if err := os.RemoveAll(src); err != nil && rvErr == nil {
				rvErr = err
			}
		}()
		cmd := os_exec.CommandContext(ctx, "python3", "-u", creationScript, "-t", src)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
		fmt.Printf("Running: %s %s\n", cmd.Path, strings.Join(cmd.Args, " "))
		if err := cmd.Run(); err != nil {
			return skerr.Wrap(err)
		}
		fmt.Println("Finished running asset creation script.")
	}

	// Find the next version number.
	fmt.Println("Finding available asset versions...")
	instances, err := getAvailableVersions(ctx, cipdClient, name)
	if err != nil {
		return skerr.Wrap(err)
	}
	fmt.Printf("Found %d package instances.\n", len(instances))
	highestVersion := -1
	for version := range instances {
		if version > highestVersion {
			highestVersion = version
		}
	}
	nextVersion := highestVersion + 1

	// If --dry-run was provided, quit now.
	if dryRun {
		fmt.Printf("--dry-run was specified; not uploading package version %d\n", nextVersion)
		return nil
	}

	// Create the new package instance.
	refs := []string{"latest"}
	tags := append([]string{
		fmt.Sprintf(tagVersionTmpl, nextVersion),
		tagProject,
	}, extraTags...)
	packagePath := fmt.Sprintf(cipdPackageNameTmpl, name)
	fmt.Printf("Uploading %s\n", packagePath)
	uploadTracker.Start()
	defer uploadTracker.Stop()
	if _, err := cipdClient.Create(ctx, packagePath, src, settings.InstallMode, skipFilesRegex, refs, tags, nil); err != nil {
		return skerr.Wrap(err)
	}

	// Write the new version file.
	if err := setVersion(name, nextVersion); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}

// cmdSetVersion implements the "set-version" sub-command.
func cmdSetVersion(ctx context.Context, name string, version int, local bool) error {
	cipdClient, _, _, err := getCIPDClient(ctx, ".", local)
	if err != nil {
		return skerr.Wrap(err)
	}

	// Ensure that the requested version actually exists.
	instances, err := getAvailableVersions(ctx, cipdClient, name)
	if err != nil {
		return skerr.Wrap(err)
	}
	if _, ok := instances[version]; !ok {
		return skerr.Fmt("Version %d not found. Available versions:\n%s\n\n", version, printVersions(instances))
	}

	// Update the version file for the asset.
	if err := setVersion(name, version); err != nil {
		if os.IsNotExist(skerr.Unwrap(err)) {
			// If the version file doesn't exist (eg. this is an asset which is
			// managed in another repo), offer to create it.
			fmt.Printf("Entry for asset %q does not exist. Create it? (y/n): ", name)
			read, err := bufio.NewReader(os.Stdin).ReadString('\n')
			if err != nil {
				return skerr.Wrap(err)
			}
			read = strings.TrimSpace(read)
			if read == "y" {
				if err := cmdAdd(ctx, name); err != nil {
					return skerr.Wrap(err)
				}
				return setVersion(name, version)
			}
		}
	}
	return nil
}

// cmdListVersions implements the "list-versions" subcommand.
func cmdListVersions(ctx context.Context, name string, local bool) error {
	cipdClient, _, _, err := getCIPDClient(ctx, ".", local)
	if err != nil {
		return skerr.Wrap(err)
	}
	instances, err := getAvailableVersions(ctx, cipdClient, name)
	if err != nil {
		return skerr.Wrap(err)
	}
	fmt.Println(printVersions(instances))
	return nil
}

// getAssetDir finds the directory for the asset within the current repo.
func getAssetDir(name string) (string, error) {
	// Note: this implementation looks suspiciously like that of
	// repo_root.GetLocal(). The difference is that this implementation looks
	// for the infra/bots directory sequence without requiring us to be inside
	// a Git checkout.
	cwd, err := os.Getwd()
	if err != nil {
		return "", skerr.Wrap(err)
	}
	assetsDirPath := filepath.Join("infra", "bots", "assets")
	for {
		assetsDir := filepath.Join(cwd, assetsDirPath)
		if st, err := os.Stat(assetsDir); err == nil && st.IsDir() {
			return filepath.Join(assetsDir, name), nil
		}
		newCwd, err := filepath.Abs(filepath.Join(cwd, ".."))
		if err != nil {
			return "", skerr.Wrap(err)
		}
		if newCwd == cwd {
			return "", skerr.Fmt("No %s dir found up to %s; are we running inside a checkout?", assetsDirPath, cwd)
		}
		cwd = newCwd
	}
}

// getVersionFilePath finds the path to the version file for the asset within
// the current repo.
func getVersionFilePath(name string) (string, error) {
	assetDir, err := getAssetDir(name)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return filepath.Join(assetDir, versionFileBaseName), nil
}

// getVersion reads the version file for the asset and returns the version
// number.
func getVersion(name string) (int, error) {
	versionFile, err := getVersionFilePath(name)
	if err != nil {
		return -1, skerr.Wrap(err)
	}
	b, err := os.ReadFile(versionFile)
	if err != nil {
		if os.IsNotExist(err) {
			return -1, skerr.Wrapf(err, "unknown asset %q", name)
		}
		return -1, skerr.Wrap(err)
	}
	version, err := strconv.Atoi(strings.TrimSpace(string(b)))
	if err != nil {
		return -1, skerr.Wrap(err)
	}
	return version, nil
}

// setVersion writes the version file for the asset.
func setVersion(name string, version int) error {
	versionFile, err := getVersionFilePath(name)
	if err != nil {
		return skerr.Wrap(err)
	}
	if err := os.WriteFile(versionFile, []byte(strconv.Itoa(version)), os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

// assetSettings contains settings for an asset.
type assetSettings struct {
	InstallMode pkg.InstallMode `json:"installMode"`
}

func defaultAssetSettings() *assetSettings {
	return &assetSettings{
		InstallMode: pkg.InstallModeSymlink,
	}
}

// getSettingsFilePath finds the path to the settings file for the given asset
// within the current repo.
func getSettingsFilePath(name string) (string, error) {
	assetDir, err := getAssetDir(name)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return filepath.Join(assetDir, assetSettingsFileBaseName), nil
}

// readAssetSettings reads the settings for the given asset. If the settings
// file does not exist, a default is returned.
func readAssetSettings(name string) (*assetSettings, error) {
	settingsFilePath, err := getSettingsFilePath(name)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	rv := defaultAssetSettings()
	if err := util.WithReadFile(settingsFilePath, func(f io.Reader) error {
		return skerr.Wrap(json.NewDecoder(f).Decode(rv))
	}); os.IsNotExist(err) {
		return rv, nil
	} else if err != nil {
		return nil, skerr.Wrap(err)
	}
	return rv, nil
}

// writeAssetSettings write the settings for the given asset.
func writeAssetSettings(name string, settings *assetSettings) error {
	settingsFilePath, err := getSettingsFilePath(name)
	if err != nil {
		return skerr.Wrap(err)
	}
	return skerr.Wrap(util.WithWriteFile(settingsFilePath, func(w io.Writer) error {
		return skerr.Wrap(json.NewEncoder(w).Encode(settings))
	}))
}

// getAvailableVersions retrieves all uploaded versions of the asset and returns
// a map of version number to InstanceDescription
func getAvailableVersions(ctx context.Context, cipdClient cipd.CIPDClient, name string) (map[int]*cipd_api.InstanceDescription, error) {
	packagePath := fmt.Sprintf(cipdPackageNameTmpl, name)
	iter, err := cipdClient.ListInstances(ctx, packagePath)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	fmt.Println("  Created iterator.")

	// Iterate the package instances.
	rv := map[int]*cipd_api.InstanceDescription{}
	for {
		infos, err := iter.Next(ctx, 10)
		if err != nil {
			// TODO(borenet): Why doesn't the code work?
			if status.Code(err) == codes.NotFound || strings.Contains(err.Error(), "no such package") {
				// There aren't any instances of this package yet.
				fmt.Println("  No instances of this package exist yet. Stopping.")
				break
			} else {
				fmt.Println("  Encountered error while iterating. Stopping.")
				return nil, skerr.Wrap(err)
			}
		}
		if len(infos) == 0 {
			fmt.Println("  Finished iterating.")
			break
		}
		fmt.Println("  Processing batch of package instances.")
		for _, info := range infos {
			fmt.Printf("    Processing: %s\n", info.Pin.InstanceID)
			// Retrieve the details for the instance, which include the tags.
			instance, err := cipdClient.DescribeInstance(ctx, info.Pin, &cipd_api.DescribeInstanceOpts{
				DescribeTags: true,
			})
			if err != nil {
				fmt.Println("  Encountered error while processing package instance.  Stopping.")
				return nil, skerr.Wrap(err)
			}
			for _, tag := range instance.Tags {
				// If this is a version tag, parse the version number out and
				// add an entry to the map.  Note that we may have multiple
				// version tags on a given package instance, eg. if we uploaded
				// a package again but its contents didn't change.
				m := tagVersionRegex.FindStringSubmatch(tag.Tag)
				if len(m) == 2 {
					version, err := strconv.Atoi(m[1])
					if err != nil {
						return nil, skerr.Wrap(err)
					}
					rv[version] = instance
				}
			}
		}
	}
	return rv, nil
}

// printVersions returns a human-friendly string describing the given package
// instances.
func printVersions(instances map[int]*cipd_api.InstanceDescription) string {
	versions := make([]int, 0, len(instances))
	for version := range instances {
		versions = append(versions, version)
	}
	sort.Ints(versions)
	strs := make([]string, 0, len(versions))
	for _, version := range versions {
		instance := instances[version]
		shortID := instance.Pin.InstanceID[:12]
		strs = append(strs, fmt.Sprintf("%d.\t%s...\t%s", version, shortID, instance.RegisteredTs))
	}
	return strings.Join(strs, "\n")
}
