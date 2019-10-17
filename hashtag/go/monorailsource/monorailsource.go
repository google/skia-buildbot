package monorailsource

import (
	"time"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/issues"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
)

type monorailSource struct {
	m issues.IssueTracker
}

// New returns a new Source.
func New() (source.Source, error) {
	ts, err := auth.NewDefaultTokenSource(true, "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to create tokenSource")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	m := issues.NewMonorailIssueTracker(c, issues.PROJECT_SKIA)

	return &monorailSource{
		m: m,
	}, nil
}

func (m *monorailSource) ByHashtag(hashtag string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)
		matchingIssues, err := m.m.FromQuery(hashtag)
		if err != nil {
			sklog.Errorf("Failed to build Monorail search: %s", err)
			return
		}
		for _, issue := range matchingIssues {
			ret <- source.Artifact{
				Title:        issue.Title,
				URL:          "",
				LastModified: time.Now(),
				Kind:         source.Monorail,
			}
		}
	}()

	return ret
}
func (m *monorailSource) ByUser(string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	close(ret)
	return ret
}
