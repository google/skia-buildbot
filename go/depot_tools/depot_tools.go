package depot_tools

/*
   Utility for finding a depot_tools checkout.
*/

import (
	"bufio"
	"bytes"
	"fmt"
	"regexp"

	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/git"
)

var (
	// Filled in by gen_version.go.
	DEPOT_TOOLS_VERSION = "none"
)

// Sync syncs the depot_tools checkout to DEPOT_TOOLS_VERSION. Returns the
// location of the checkout or an error.
func Sync(workdir string) (string, error) {
	// Clone the repo if necessary.
	co, err := git.NewCheckout(common.REPO_DEPOT_TOOLS, workdir)
	if err != nil {
		return "", err
	}

	// Avoid doing any syncing if we already have the desired revision.
	hash, err := co.RevParse("HEAD")
	if err != nil {
		return "", err
	}
	if hash == DEPOT_TOOLS_VERSION {
		return co.Dir(), nil
	}

	// Sync the checkout into the desired state.
	if err := co.Fetch(); err != nil {
		return "", fmt.Errorf("Failed to fetch repo in %s: %s", co.Dir(), err)
	}
	if err := co.Cleanup(); err != nil {
		return "", fmt.Errorf("Failed to cleanup repo in %s: %s", co.Dir(), err)
	}
	if _, err := co.Git("reset", "--hard", DEPOT_TOOLS_VERSION); err != nil {
		return "", fmt.Errorf("Failed to reset repo in %s: %s", co.Dir(), err)
	}
	hash, err = co.RevParse("HEAD")
	if err != nil {
		return "", err
	}
	if hash != DEPOT_TOOLS_VERSION {
		return "", fmt.Errorf("Got incorrect depot_tools revision: %s", hash)
	}
	return co.Dir(), nil
}

const (
	// DEPSSkiaVarRegEx is the default regular expression to extract the
	// commit hash from a DEPS file when is defined as a variable.
	DEPSSkiaVarRegEx = "^.*'skia_revision'.*:.*'([0-9a-f]+)'.*$"

	// DEPSSkiaURLRegEx is the default regular expression to extract the
	// commit hash from a DEPS file when it is defined as a URL.
	DEPSSkiaURLRegEx = "^.*http.*://.*/skia/?@([0-9a-f]+).*$"
)

// DEPSExtractor defines a simple interface to extract a commit hash from
// a DEPS file.
type DEPSExtractor interface {
	// ExtractCommit extracts the commit has from a DEPS file. The first argument
	// is the content of the DEPS file. The second argument allows to call this
	// function by passing the results of a read operaiton, e.g.:
	//    ExtractCommit(gitdir.GetFile("DEPS", commitHash))
	// If err is not nil it will simply be returned. If the commit cannot
	// be extracted an error is returned.
	ExtractCommit(DEPSContent string, err error) (string, error)
}

// NewRegExDEPSExtractor returns a new DEPSExtractor based on a regular expression.
func NewRegExDEPSExtractor(regEx string) DEPSExtractor {
	return &regExDEPSExtractor{
		regEx: regexp.MustCompile(regEx),
	}
}

type regExDEPSExtractor struct {
	regEx *regexp.Regexp
}

// ExtractCommit implments the DEPSExtractor interface.
func (r *regExDEPSExtractor) ExtractCommit(content string, err error) (string, error) {
	if err != nil {
		return "", err
	}

	scanner := bufio.NewScanner(bytes.NewBuffer([]byte(content)))
	for scanner.Scan() {
		line := scanner.Text()
		result := r.regEx.FindStringSubmatch(line)
		if len(result) == 2 {
			return result[1], nil
		}
	}
	return "", fmt.Errorf("Regex does not match any line in DEPS file.")
}
