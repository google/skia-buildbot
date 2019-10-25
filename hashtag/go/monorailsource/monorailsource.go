package monorailsource

import (
	"context"
	"fmt"
	"time"

	"github.com/spf13/viper"
	"go.skia.org/infra/go/monorail/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
)

const monorailTimeFormat = "2006-01-02T15:04:05"

// monorailSource implements source.Source.
type monorailSource struct {
	m          *monorail.Service
	projectID  string
	linkFormat string
	sort       string
}

// New returns a new Source.
func New() (source.Source, error) {
	m, err := monorail.NewService(context.Background())
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to create monorail service.")
	}

	return &monorailSource{
		m:          m,
		projectID:  viper.GetString("sources.monorail.projectID"),
		linkFormat: viper.GetString("sources.monorail.linkFormat"),
		sort:       viper.GetString("sources.monorail.sort"),
	}, nil
}

// See source.Source.
func (m *monorailSource) ByHashtag(hashtag string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)
		matchingIssues, err := m.m.Issues.List(m.projectID).Q(hashtag).Sort(m.sort).Do()
		if err != nil {
			sklog.Errorf("Failed to build Monorail search: %s", err)
			return
		}
		for _, issue := range matchingIssues.Items {
			ts, err := time.Parse(monorailTimeFormat, issue.StatusModified)
			if err != nil {
				sklog.Errorf("Can't parse %q at time: %s", issue.StatusModified, err)
				ts = time.Now()
			}
			ret <- source.Artifact{
				Title:        issue.Title,
				URL:          fmt.Sprintf(m.linkFormat, issue.Id),
				LastModified: ts,
			}
		}
	}()

	return ret
}

// See source.Source.
func (m *monorailSource) ByUser(string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	close(ret)
	return ret
}
