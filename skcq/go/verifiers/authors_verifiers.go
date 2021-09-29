package verifiers

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/skcq/go/codereview"
	"go.skia.org/infra/skcq/go/types"
)

var emailsRegex = regexp.MustCompile(` <(.*)>`)

// NewAuthorsVerifier returns an instance of AuthorsVerifier.
func NewAuthorsVerifier(cr codereview.CodeReview, authorsFileContent string) (types.Verifier, error) {
	return &AuthorsVerifier{authorsFileContent, cr}, nil
}

// AuthorsVerifier implements the types.Verifier interface.
type AuthorsVerifier struct {
	authorsFileContent string
	cr                 codereview.CodeReview
}

// Name implements the types.Verifier interface.
func (wv *AuthorsVerifier) Name() string {
	return "AuthorsVerifier"
}

// Verify implements the types.Verifier interface.
func (wv *AuthorsVerifier) Verify(ctx context.Context, ci *gerrit.ChangeInfo, startTime int64) (state types.VerifierState, reason string, err error) {

	// Get the author of the latest patch of the specified change.
	author, err := wv.cr.GetCommitAuthor(ctx, ci.Issue, "current")
	if err != nil {
		return "", "", skerr.Wrapf(err, "Could not find author of %d/current", ci.Issue)
	}

	// Parse the AUTHORS file content and find all email regexes in them.
	for _, l := range strings.Split(wv.authorsFileContent, "\n") {
		l = strings.TrimSpace(l)
		if l != "" && !strings.HasPrefix(l, "#") {
			m := emailsRegex.FindStringSubmatch(l)
			if len(m) == 2 {
				// This uses shell patterns like '*@email.com'. Change to
				// regex supported string like ".*@gmail.com"
				emailRegexStr := strings.Replace(m[1], "*", ".*", -1)
				emailRegex, err := regexp.Compile("^" + emailRegexStr + "$")
				if err != nil {
					sklog.Errorf("[%d] Could not parse \"%s\" in the AUTHORS file: %s", ci.Issue, emailRegexStr, err)
				}
				if emailRegex.MatchString(author) {
					return types.VerifierSuccessState, fmt.Sprintf("The author \"%s\" is in the AUTHORS file: \"%s\"", author, l), nil
				}
			}
		}
	}

	return types.VerifierFailureState, fmt.Sprintf("The author \"%s\" was not found in the AUTHORS file", author), nil
}

// Cleanup implements the types.Verifier interface.
func (wv *AuthorsVerifier) Cleanup(ctx context.Context, ci *gerrit.ChangeInfo, cleanupPatchsetID int64) {
	return
}
