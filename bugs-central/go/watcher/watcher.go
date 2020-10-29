package watcher

// Watches different clients+issue frameworks for:
// * Open issues assigned to a rotation alias. The issue is then re-assigned to
//   the person on rotation.
// * Maybe in the future will be used to ping issues beyond SLO.

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"golang.org/x/oauth2"

	"go.skia.org/infra/bugs-central/go/bugs"
	"go.skia.org/infra/bugs-central/go/bugs/monorail"
	"go.skia.org/infra/bugs-central/go/types"
	"go.skia.org/infra/go/baseapp"
	"go.skia.org/infra/go/cleanup"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
)

const (
	// All recognized clients.
	SkiaClient types.RecognizedClient = "Skia"
)

var (
	aliasToRotation = map[string]string{
		"skia-gpu-wrangler@google.com": "https://tree-status.skia.org/current-wrangler",
	}
)

// Watcher will be used to watch for specific issues in the different issue frameworks.
type Watcher struct {
	httpClient               *http.Client
	pathToServiceAccountFile string
}

// New returns an instance of Watcher.
func New(ctx context.Context, ts oauth2.TokenSource, pathToServiceAccountFile string) (*Watcher, error) {
	httpClient := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()

	return &Watcher{
		httpClient:               httpClient,
		pathToServiceAccountFile: pathToServiceAccountFile,
	}, nil
}

// Start watches different clients+issue frameworks for:
// * Open issues assigned to a rotation alias. The issue is then re-assigned to
//   the person on rotation.
// * Maybe in the future will be used to ping issues beyond SLO.
func (p *Watcher) Start(ctx context.Context, pollInterval time.Duration) error {

	// Instantiate the bug frameworks with the different client configurations and then poll them.
	bugFrameworks := []bugs.BugFramework{}

	//////////////////// Skia - Monorail - Wrangler reassigner ////////////////////
	wranglerQueryConfig := &monorail.MonorailQueryConfig{
		Instance: "skia",
		Query:    "is:open owner:skia-gpu-wrangler@google.com",
		Client:   SkiaClient,
	}
	skMonorailWrangler, err := monorail.New(ctx, p.pathToServiceAccountFile, nil, wranglerQueryConfig)
	if err != nil {
		return skerr.Wrapf(err, "failed to init monorail for skia")
	}
	bugFrameworks = append(bugFrameworks, skMonorailWrangler)

	cleanup.Repeat(pollInterval, func(ctx context.Context) {
		if !*baseapp.Local {
			// Ignore the passed-in context; this allows us to continue running even if the
			// context is canceled due to transient errors.
			ctx = context.Background()
		}

		issues, _, err := skMonorailWrangler.Search(ctx)
		if err != nil {
			sklog.Errorf("Error when searching wrangler issues: %s", err)
			return
		}

		fmt.Println("FOUND THESE ISSUES")
		for _, i := range issues {
			fmt.Println(i.Link)
		}
	}, nil)

	return nil
}
