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

// addQuery extends the monorail IssuesListCall based on the source.Query.
func (m *monorailSource) addQuery(listCall *monorail.IssuesListCall, q source.Query) *monorail.IssuesListCall {
	if q.Type == source.HashtagQuery {
		listCall = listCall.Q(q.Value)
	} else if q.Type == source.UserQuery {
		listCall = listCall.Owner(q.Value)
	}
	if !q.Begin.IsZero() {
		listCall = listCall.UpdatedMin(q.Begin.Unix())
	}
	if !q.End.IsZero() {
		listCall = listCall.UpdatedMax(q.End.Unix())
	}
	return listCall
}

// See source.Source.
func (m *monorailSource) Search(ctx context.Context, q source.Query) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)
		listCall := m.m.Issues.List(m.projectID).Context(ctx).Sort(m.sort)
		listCall = m.addQuery(listCall, q)
		matchingIssues, err := listCall.Do()
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
