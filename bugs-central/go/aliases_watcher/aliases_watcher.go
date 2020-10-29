// Package aliases_watcher watches different clients+issue frameworks for open issues assigned to
// a rotation alias. The issue is then re-assigned to the person on rotation.
package aliases_watcher

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/bugs-central/go/bugs/monorail"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/rotations"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

var (
	aliasCommentTemplate = `Issue update by bugs-central.skia.org\n\nAuto-assigning the issue from %s to the current %s from %s.`
)

type aliasToWatch struct {
	rotationAlias string
	rotationURL   string
	comment       string
	bugFramework  bugs.BugFramework
}

// AliasesWatcher will be used to watch for specific issues in the different issue frameworks.
type AliasesWatcher struct {
	httpClient               *http.Client
	pathToServiceAccountFile string
}

// New returns an instance of AliasesWatcher.
// ts needs auth.SCOPE_USERINFO_EMAIL scope. It will be used to create an http client for getting
// rotations and calling the monorail API.
// pathToServiceAccountFile is the path to the service account's JSON file. It will be used for
// monorail API auth.
func New(ctx context.Context, ts oauth2.TokenSource, pathToServiceAccountFile string) (*AliasesWatcher, error) {
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	return &AliasesWatcher{
		httpClient:               httpClient,
		pathToServiceAccountFile: pathToServiceAccountFile,
	}, nil
}

// Start watches different clients+issue frameworks for open issues assigned to a rotation alias.
// The issue is then re-assigned to the person on rotation.
// It hardcodes information about the rotations and aliases that Skia cares about. It may be
// possible to extract some/all of these into flags or YAML config files in the future.
func (p *AliasesWatcher) Start(ctx context.Context, pollInterval time.Duration) error {

	// Collect all aliases to watch and then poll them.
	aliasesToWatch := []aliasToWatch{}

	//////////////////// Skia - Monorail - Wrangler reassigner ////////////////////
	wranglerAlias := "skia-gpu-wrangler@google.com"
	wranglerRotation := "https://tree-status.skia.org/current-wrangler"
	wranglerQueryConfig := &monorail.MonorailQueryConfig{
		Instance: "skia",
		Query:    "is:open owner:skia-gpu-wrangler@google.com",
		Client:   "Skia",
	}
	skMonorailWrangler, err := monorail.New(ctx, p.pathToServiceAccountFile, nil, wranglerQueryConfig)
	if err != nil {
		return skerr.Wrapf(err, "failed to init monorail for skia")
	}
	aliasesToWatch = append(aliasesToWatch, aliasToWatch{
		rotationAlias: wranglerAlias,
		rotationURL:   wranglerRotation,
		comment:       fmt.Sprintf(aliasCommentTemplate, wranglerAlias, "wrangler", wranglerRotation),
		bugFramework:  skMonorailWrangler,
	})

	cleanup.Repeat(pollInterval, func(_ context.Context) {
		// Using context.Background since this is a background task.
		ctx = context.Background()

		for _, a := range aliasesToWatch {
			issues, _, err := a.bugFramework.Search(ctx)
			if err != nil {
				sklog.Errorf("Error when searching issues: %s", err)
				return
			}
			emails, err := rotations.FromURL(p.httpClient, a.rotationURL)
			if err != nil {
				sklog.Errorf("Error getting the current rotation: %s", err)
				return
			}
			// Skia rotations always have exactly one person on each rotation.
			if len(emails) != 1 {
				sklog.Errorf("Got %d emails from %s. Expected 1.", len(emails), a.rotationURL)
				return
			}
			rotationEmail := emails[0]

			for _, i := range issues {
				sklog.Info("Replacing owner in %s from %s to %s", i.Id, a.rotationAlias, rotationEmail)
				if err := a.bugFramework.SetOwnerAndAddComment(rotationEmail, a.comment, i.Id); err != nil {
					sklog.Errorf("Could not set owner %s and add comment to %s", rotationEmail, i.Id)
					return
				}
			}
		}
	}, nil)

	return nil
}
