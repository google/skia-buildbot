package monorailsource

import (
	"context"
	"io/ioutil"
	"time"

	"go.skia.org/infra/go/monorail/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
	"golang.org/x/oauth2/google"
)

type monorailSource struct {
	m *monorail.Service
}

// New returns a new Source.
func New() (source.Source, error) {

	c, err := google.DefaultClient(context.Background(), "https://www.googleapis.com/auth/userinfo.email")
	if err != nil {
		return nil, skerr.Wrapf(err, "Unable to create tokenSource")
	}
	resp, err := c.Get("https://monorail-prod.appspot.com/_ah/api/monorail/v1/projects/skia/issues?alt=json&prettyPrint=false&q=skottie")
	sklog.Infof("%#v %s", *resp, err)
	b, err := ioutil.ReadAll(resp.Body)
	sklog.Infof("%s %s", string(b), err)
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
		matchingIssues, err := m.m.Issues.List("skia").Q(hashtag).Do()
		if err != nil {
			sklog.Errorf("Failed to build Monorail search: %s", err)
			return
		}
		for _, issue := range matchingIssues.Items {
			ts, err := time.Parse(time.RFC3339, issue.StatusModified)
			if err != nil {
				sklog.Errorf("Can't parse %q at time: %s", issue.StatusModified, err)
				ts = time.Now()
			}
			ret <- source.Artifact{
				Title:        issue.Title,
				URL:          "",
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
