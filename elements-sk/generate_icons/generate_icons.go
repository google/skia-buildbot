// This program generates custom elements for various SVG icons found in the material-design-icons
// NPM package (https://github.com/google/material-design-icons). All generated custom elements
// will be placed in directories following the //elements-sk/modules/icons/<icon-name>-sk naming
// scheme.
//
// This program also generates file //elements-sk/modules/icon-demo-sk/icons.ts, which is used by
// the icons-demo-sk custom element to showcase all icons.
//
// Usage:
//
//	$ bazelisk run --config=mayberemote //elements-sk/generate_icons
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"go.skia.org/infra/elements-sk/generate_icons/demo"
	"go.skia.org/infra/elements-sk/generate_icons/icon"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/untar"
)

var iconCategories = []string{
	"action",
	"alert",
	"av",
	"communication",
	"content",
	"device",
	"editor",
	"file",
	"hardware",
	"image",
	"maps",
	"navigation",
	"notification",
	"places",
	"social",
	"toggle",
}

// We download the material-design-icons NPM package manually, rather than listing it in
// //package.json, because we don't expect to run this program often, and said package is not used
// anywhere else in our codebase.
const materialDesignIconsNPMPackageURL = "https://registry.npmjs.org/material-design-icons/-/material-design-icons-3.0.1.tgz"

// There is one directory per icon category within the NPM package.
const iconCategoryDirTmpl = "package/%s/svg/production/"

// Each icon has multiple associated files (e.g. different sizes), but we only care about one.
var iconFileNameRegexp = regexp.MustCompile(`(ic_)?(?P<name>.+)_24px.svg`)

func main() {
	// Get the path to the repository root (and ensure we are running under Bazel).
	workspaceDir := os.Getenv("BUILD_WORKSPACE_DIRECTORY")
	if workspaceDir == "" {
		sklog.Fatal("The BUILD_WORKSPACE_DIRECTORY environment variable is not set. Are you running this program via Bazel?")
	}

	// Download material-design-icons NPM package.
	materialDesignIconsDir, cleanupFn, err := downloadAndExtractMaterialDesignIconsNPMPackage()
	if err != nil {
		sklog.Fatalf("Error downloading/extracting the material-design-icons NPM package: %s", err)
	}
	defer cleanupFn()

	// The icons-demo-sk custom element will display icons grouped by category.
	iconNamesByCategory := map[string][]string{}

	// For simplicity, all our icon elements are generated within the same directory. However, some
	// icon names appear in multiple categories, causing a file name clash in the directory where
	// icon elements are generated. for example:
	//
	// - https://github.com/google/material-design-icons/blob/3.0.1/places/svg/production/ic_rv_hookup_24px.svg
	// - https://github.com/google/material-design-icons/blob/3.0.1/notification/svg/production/ic_rv_hookup_24px.svg
	//
	// To remedy this, we only generate an icon element for the first ocurrence of a given icon
	// name.
	allIconNames := map[string]bool{}

	// Ensure that we'll scan icon category directories alphabetically.
	sort.Strings(iconCategories)

	for _, iconCategory := range iconCategories {
		iconNamesByCategory[iconCategory] = []string{}

		// Scan directory for the current icon category.
		iconsDirPath := filepath.Join(materialDesignIconsDir, fmt.Sprintf(iconCategoryDirTmpl, iconCategory))
		sklog.Infof("Scanning directory: %s", iconsDirPath)
		files, err := os.ReadDir(iconsDirPath)
		if err != nil {
			sklog.Fatalf("Error reading from directory %s: %s", iconsDirPath, err)
		}

		// Iterate over all files in the current icon category directory.
		for _, file := range files {
			if match := iconFileNameRegexp.FindStringSubmatch(file.Name()); match != nil {
				iconName := sanitizeIconName(match[2])

				// Ignore file if we have already observed an icon with the same name.
				if allIconNames[iconName] {
					continue
				}

				allIconNames[iconName] = true
				iconNamesByCategory[iconCategory] = append(iconNamesByCategory[iconCategory], iconName)

				// Generate a custom element for the current icon.
				iconSvgPath := filepath.Join(iconsDirPath, file.Name())
				err := icon.Generate(workspaceDir, iconName, iconSvgPath)
				if err != nil {
					sklog.Fatalf("Error generating custom element for icon file %s: %s", iconSvgPath, err)
				}
				sklog.Infof("Generated custom element for icon: %s", iconName)
			}
		}

		// Sort icons by name, as sanitization might have broken the lexicographic order
		// (e.g. "3d" -> "three-d").
		sort.Strings(iconNamesByCategory[iconCategory])
	}

	// Generate the icons.ts file used by the icons-demo-sk custom element.
	if err := demo.Generate(workspaceDir, iconNamesByCategory); err != nil {
		sklog.Fatalf("Error icons.ts: %s", err)
	}
}

// downloadAndExtractMaterialDesignIconsNPMPackage downloads the material-design-icons NPM package
// and extracts the archive in a temporary directory. It returns the path to the directory with the
// extracted archive and a cleanup function to delete said directory.
func downloadAndExtractMaterialDesignIconsNPMPackage() (string, func(), error) {
	// Create a temporary directory where we will extract the downloaded archive.
	materialDesignIconsDir, err := os.MkdirTemp("/tmp", "material-design-icons-*")
	if err != nil {
		sklog.Fatalf("Error creating temporary directory: %s", err)
	}
	cleanupFn := func() {
		if err := os.RemoveAll(materialDesignIconsDir); err != nil {
			sklog.Infof("Error deleting temporary directory: %s", err)
		}
	}

	// Download NPM package.
	sklog.Infof("Downloading material-design-icons NPM package from: %s", materialDesignIconsNPMPackageURL)
	resp, err := httputils.NewTimeoutClient().Get(materialDesignIconsNPMPackageURL)
	if err != nil {
		cleanupFn()
		return "", func() {}, skerr.Wrap(err)
	}
	defer resp.Body.Close()

	// Extract NPM package.
	if err := untar.Untar(resp.Body, materialDesignIconsDir); err != nil {
		cleanupFn()
		return "", func() {}, skerr.Wrap(err)
	}

	return materialDesignIconsDir, cleanupFn, nil
}

func sanitizeIconName(s string) string {
	s = strings.Replace(s, "_", "-", -1)
	if strings.HasPrefix(s, "3d") {
		s = strings.Replace(s, "3d", "three-d", -1)
	}
	return s
}
