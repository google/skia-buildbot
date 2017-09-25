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
	// DefaultSkiaDEPSRegexp is the default regular expression to extract the
	// commit hash from a DEPS file.
	DefaultSkiaDEPSRegexp = "^.*'skia_revision'.*:.*'([0-9a-f]+)'.*$"

	DefautlVarExtractRegEx = ""
	DefaultURLExtractRegEx = ""
)

type DEPSExtractor interface {
	ExtractCommit(DEPSContent string, err error) string
}

func NewRegExDEPSExtractor(regEx string) DEPSExtractor {
	return &regExDEPSExtractor{
		regEx: regexp.MustCompile(regEx),
	}
}

type regExDEPSExtractor struct {
	regEx *regexp.Regexp
}

func (r *regExDEPSExtractor) ExtractCommit(content string, err error) string {
	if err != nil {
		return ""
	}

	scanner := bufio.NewScanner(bytes.NewBuffer([]byte(content)))
	for scanner.Scan() {
		line := scanner.Text()
		result := r.regEx.FindStringSubmatch(line)
		if len(result) == 2 {
			return result[1]
		}
	}
	return ""
}
