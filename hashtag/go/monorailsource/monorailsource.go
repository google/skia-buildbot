package monorailsource

import (
	"context"
	"fmt"
	"time"

	"go.skia.org/infra/go/monorail/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
)

type monorailSource struct {
	m *monorail.Service
}

// New returns a new Source.
func New() (source.Source, error) {
	m, err := monorail.NewService(context.Background())
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to create tokenSource")
	}

	return &monorailSource{
		m: m,
	}, nil
}

func (m *monorailSource) ByHashtag(hashtag string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)
		matchingIssues, err := m.m.Issues.List("skia").Q(hashtag).Sort("-Id").Do()
		if err != nil {
			sklog.Errorf("Failed to build Monorail search: %s", err)
			return
		}
		for _, issue := range matchingIssues.Items {
			ts, err := time.Parse("2006-01-02T15:04:05", issue.StatusModified)
			if err != nil {
				sklog.Errorf("Can't parse %q at time: %s", issue.StatusModified, err)
				ts = time.Now()
			}
			ret <- source.Artifact{
				Title:        issue.Title,
				URL:          fmt.Sprintf("https://bugs.skia.org/%d", issue.Id),
				LastModified: ts,
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
