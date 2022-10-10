// This program takes as inputs one or more TypeScript source files, determines whether they depend
// on any elements-sk modules, and writes to standard output a Sass stylesheet that imports the
// stylesheets required by any such elements-sk module.
//
// This program is used by the sk_element and sk_page Bazel rules to automatically generate a Sass
// stylesheet with any necessary elements-sk imports. This is necessary because the underlying
// Bazel rules ignore Webpack-style Sass imports from TypeScript files.
package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"go.skia.org/infra/bazel/gazelle/frontend/parsers"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

// knownStylesheets is the list of Sass stylesheets we would like to import automatically based on
// the TypeScript imports found in a TypeScript source file.
//
// This list can be regenerated with the following command, which must be run from an elements-sk
// repository checkout:
//
//     $ find src | grep \.scss$ | sed -E "s/^src/elements-sk/" | sort
var knownStylesheets = []string{
	"elements-sk/checkbox-sk/checkbox-sk.scss",
	"elements-sk/collapse-sk/collapse-sk.scss",
	"elements-sk/colors.scss",
	"elements-sk/error-toast-sk/error-toast-sk.scss",
	"elements-sk/icon/icon-sk.scss",
	"elements-sk/multi-select-sk/multi-select-sk.scss",
	"elements-sk/nav-links-sk/nav-links-sk.scss",
	"elements-sk/radio-sk/radio-sk.scss",
	"elements-sk/select-sk/select-sk.scss",
	"elements-sk/spinner-sk/spinner-sk.scss",
	"elements-sk/styles/buttons/buttons.scss",
	"elements-sk/styles/select/select.scss",
	"elements-sk/styles/table/table.scss",
	"elements-sk/tabs-panel-sk/tabs-panel-sk.scss",
	"elements-sk/tabs-sk/tabs-sk.scss",
	"elements-sk/themes/color-palette.scss",
	"elements-sk/themes/themes.scss",
	"elements-sk/toast-sk/toast-sk.scss",
}

// elementsSkStylesheetsFromTsImport takes the verbatim path of a TypeScript import statement, and
// if the path corresponds to an elements-sk module, returns the list of Sass stylesheets required
// by the elements-sk module, or nil otherwise.
func elementsSkStylesheetsFromTsImport(tsImport string) []string {
	var stylesheets []string

	// Heuristic: if a TypeScript import is prefixed by the parent directory of a known
	// stylesheet, we assume that the stylesheet must be imported as well.
	//
	// Example input TypeScript source file:
	//
	//     // Resolves to "elements-sk/checkbox-sk/checkbox-sk.ts".
	//     import 'elements-sk/checkbox-sk/checkbox-sk';
	//
	//     // Resolves to "elements-sk/styles/buttons/index.ts".
	//     import 'elements-sk/styles/buttons';
	//
	// Expected Sass imports:
	//
	//     @import 'node_modules/elements-sk/checkbox-sk/checkbox-sk.scss';
	//     @import 'node_modules/elements-sk/styles/buttons/buttons.scss';
	for _, stylesheet := range knownStylesheets {
		// Exclude top-level stylesheets (e.g. "elements-sk/colors.scss").
		if filepath.Dir(stylesheet) == "elements-sk" {
			continue
		}

		if strings.HasPrefix(tsImport, filepath.Dir(stylesheet)) {
			stylesheets = append(stylesheets, stylesheet)
		}
	}

	// In addition, we must account for dependencies between components. There are only a few
	// such dependencies, and they change infrequently, so we handle them in an ad-hoc way.
	if strings.HasPrefix(tsImport, "elements-sk/error-toast-sk") {
		stylesheets = append(stylesheets, "elements-sk/toast-sk/toast-sk.scss")
	}
	if strings.HasPrefix(tsImport, "elements-sk/radio-sk") {
		stylesheets = append(stylesheets, "elements-sk/checkbox-sk/checkbox-sk.scss")
	}

	return stylesheets
}

func main() {
	tsSources := os.Args[1:]
	if len(tsSources) == 0 {
		sklog.Fatalf("Usage: %s <one or more TypeScript source files>", os.Args[0])
	}

	var outputSassImports []string

	// Iterate over all the input files.
	for _, tsSource := range tsSources {
		// Read in the current TypeScript source file, and extract the paths of its import statements.
		b, err := ioutil.ReadFile(tsSource)
		if err != nil {
			sklog.Fatalf("Error while reading file %q: %v", tsSource, err)
		}
		tsImports := parsers.ParseTSImports(string(b))

		// Map each TypeScript import to zero or more elements-sk Sass imports.
		for _, tsImport := range tsImports {
			for _, stylesheet := range elementsSkStylesheetsFromTsImport(tsImport) {
				outputSassImports = append(outputSassImports, fmt.Sprintf("@import 'node_modules/%s';", stylesheet))
			}
		}
	}

	// Print out the Sass import statements for any required elements-sk stylesheets.
	outputSassImports = util.SSliceDedup(outputSassImports)
	sort.Strings(outputSassImports)
	for _, imp := range outputSassImports {
		fmt.Println(imp)
	}
}
