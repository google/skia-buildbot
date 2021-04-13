package asset

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/urfave/cli/v2"
	cipd_api "go.chromium.org/luci/cipd/client/cipd"
	"go.chromium.org/luci/cipd/client/cipd/pkg"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/git"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/repo_root"
	"go.skia.org/infra/go/skerr"
)

const (
	cipdPackageNameTmpl = "skia/bots/%s"
	cipdServiceURL      = "https://chrome-infra-packages.appspot.com"

	tagProject       = "project:skia"
	tagVersionPrefix = "version:"
	tagVersionTmpl   = tagVersionPrefix + "%d"

	creationScriptBaseName        = "create.py"
	creationScriptInitialContents = `#!/usr/bin/env python
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

	versionFileBaseName = "VERSION"
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
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 2 {
						return skerr.Fmt("Expected exactly two positional arguments.")
					}
					return cmdDownload(ctx.Context, args[0], args[1])
				},
			},
			{
				Name:        "upload",
				Description: "Upload a new version of an asset. If --in is provided, use the contents of the provided directory, otherwise run the creation script to generate the package contents.",
				Usage:       "upload <asset name>",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:  flagIn,
						Value: "",
						Usage: "Use the contents of this directory as the package contents. If not provided, expects a creation script to be present within the asset dir.",
					},
				},
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 1 {
						return skerr.Fmt("Expected exactly one positional argument.")
					}
					return cmdUpload(ctx.Context, args[0], ctx.String(flagIn))
				},
			},
			{
				Name:        "get-version",
				Description: "Print the current version of the given asset.",
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
				Description: "Manually the current version of the given asset. Typically you should use \"upload\" instead. This will cause problems if the given version has not already been uploaded.",
				Usage:       "set-version <asset name> <version>",
				Action: func(ctx *cli.Context) error {
					args := ctx.Args().Slice()
					if len(args) != 2 {
						return skerr.Fmt("Expected exactly two positional arguments.")
					}
					version, err := strconv.Atoi(args[1])
					if err != nil {
						return skerr.Wrapf(err, "invalid version number")
					}
					return setVersion(args[0], version)
				},
			},
		},
	}
}

func cmdAdd(ctx context.Context, name string) error {
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
	versionFile := filepath.Join(assetDir, versionFileBaseName)
	if err := ioutil.WriteFile(versionFile, []byte{}, os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}
	fmt.Printf("Do you want to add a creation script for this asset? (y/n): ")
	read, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return skerr.Wrap(err)
	}
	read = strings.TrimSpace(read)
	if read == "y" {
		creationScript := filepath.Join(assetDir, creationScriptBaseName)
		if err := ioutil.WriteFile(creationScript, []byte(creationScriptInitialContents), os.ModePerm); err != nil {
			return skerr.Wrap(err)
		}
		fmt.Println(fmt.Sprintf("Created %s; you will need to add implementation before uploading the asset.", creationScript))
	}
	if _, err := git.GitDir(".").Git(ctx, "add", assetDir); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func cmdRemove(ctx context.Context, name string) error {
	assetDir, err := getAssetDir(name)
	if err != nil {
		return skerr.Wrap(err)
	}
	if _, err := git.GitDir(".").Git(ctx, "rm", "-rf", assetDir); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func getCIPDClient(ctx context.Context, rootDir string) (cipd.CIPDClient, error) {
	ts, err := auth.NewDefaultTokenSource(true, auth.SCOPE_USERINFO_EMAIL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).Client()
	cipdClient, err := cipd.NewClient(httpClient, rootDir, cipd.DefaultServiceURL)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	return cipdClient, nil
}

func cmdDownload(ctx context.Context, name, dest string) error {
	cipdClient, err := getCIPDClient(ctx, dest)
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
	if err := cipdClient.FetchAndDeployInstance(ctx, "", pin, 0); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}

func cmdUpload(ctx context.Context, name, src string) (rvErr error) {
	cipdClient, err := getCIPDClient(ctx, ".")
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
		src, err = ioutil.TempDir("", "")
		if err != nil {
			return skerr.Wrap(err)
		}
		defer func() {
			if err := os.RemoveAll(src); err != nil && rvErr == nil {
				rvErr = err
			}
		}()
		cmd := []string{"python", creationScript, "-t", src}
		fmt.Println(fmt.Sprintf("Running: %s", strings.Join(cmd, " ")))
		if _, err := exec.RunCwd(ctx, ".", cmd...); err != nil {
			return skerr.Wrap(err)
		}
	}

	// Find the next version number.
	packagePath := fmt.Sprintf(cipdPackageNameTmpl, name)
	iter, err := cipdClient.ListInstances(ctx, packagePath)
	if err != nil {
		return skerr.Wrap(err)
	}
	highestVersion := -1
	for {
		infos, err := iter.Next(ctx, 10)
		if err != nil {
			// TODO(borenet): Why doesn't the code work?
			if status.Code(err) == codes.NotFound || strings.Contains(err.Error(), "no such package") {
				// There aren't any instances of this package yet.
				break
			} else {
				return skerr.Wrap(err)
			}
		}
		if len(infos) == 0 {
			break
		}
		for _, info := range infos {
			instance, err := cipdClient.DescribeInstance(ctx, info.Pin, &cipd_api.DescribeInstanceOpts{
				DescribeTags: true,
			})
			if err != nil {
				return skerr.Wrap(err)
			}
			for _, tag := range instance.Tags {
				m := tagVersionRegex.FindStringSubmatch(tag.Tag)
				if len(m) == 2 {
					version, err := strconv.Atoi(m[1])
					if err != nil {
						return skerr.Wrap(err)
					}
					if version > highestVersion {
						highestVersion = version
					}
					break
				}
			}
		}
	}
	nextVersion := highestVersion + 1

	// Create the new package instance.
	refs := []string{"latest"}
	tags := []string{
		fmt.Sprintf(tagVersionTmpl, nextVersion),
		tagProject,
	}
	if _, err := cipdClient.Create(ctx, packagePath, src, pkg.InstallModeSymlink, skipFilesRegex, refs, tags, nil); err != nil {
		return skerr.Wrap(err)
	}

	// Write the new version file.
	if err := setVersion(name, nextVersion); err != nil {
		return skerr.Wrap(err)
	}

	return nil
}

func getAssetDir(name string) (string, error) {
	repoRoot, err := repo_root.GetLocal()
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return filepath.Join(repoRoot, "infra", "bots", "assets", name), nil
}

func getVersionFilePath(name string) (string, error) {
	assetDir, err := getAssetDir(name)
	if err != nil {
		return "", skerr.Wrap(err)
	}
	return filepath.Join(assetDir, versionFileBaseName), nil
}

func getVersion(name string) (int, error) {
	versionFile, err := getVersionFilePath(name)
	if err != nil {
		return -1, skerr.Wrap(err)
	}
	b, err := ioutil.ReadFile(versionFile)
	if err != nil {
		return -1, skerr.Wrap(err)
	}
	version, err := strconv.Atoi(string(b))
	if err != nil {
		return -1, skerr.Wrap(err)
	}
	return version, nil
}

func setVersion(name string, version int) error {
	versionFile, err := getVersionFilePath(name)
	if err != nil {
		return skerr.Wrap(err)
	}
	if err := ioutil.WriteFile(versionFile, []byte(strconv.Itoa(version)), os.ModePerm); err != nil {
		return skerr.Wrap(err)
	}
	return nil
}
