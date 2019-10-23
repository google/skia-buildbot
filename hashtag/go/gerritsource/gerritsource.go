package gerritsource

import (
	"context"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
)

type gerritSource struct {
	g *gerrit.Gerrit
}

// New returns a new Source.
func New() (source.Source, error) {
	c := httputils.DefaultClientConfig().With2xxOnly().Client()
	g, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, "", c)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create gerrit instance")
	}
	return &gerritSource{
		g: g,
	}, err
}

// See source.Source.
func (g *gerritSource) ByHashtag(hashtag string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)
		terms := []*gerrit.SearchTerm{
			&gerrit.SearchTerm{
				Key:   "message",
				Value: hashtag,
			},
			&gerrit.SearchTerm{
				Key:   "-owner",
				Value: "skia-autoroll@skia-public.iam.gserviceaccount.com",
			},
			&gerrit.SearchTerm{
				Key:   "-owner",
				Value: "skia-lottie-ci-autoroll@skia-buildbots.google.com.iam.gserviceaccount.com",
			},
		}
		changes, err := g.g.Search(context.Background(), gerrit.MAX_GERRIT_LIMIT, terms...)
		if err != nil {
			sklog.Errorf("Failed to build Gerrit search: %s", err)
			return
		}
		for _, c := range changes {
			ret <- source.Artifact{
				Title:        c.Subject,
				URL:          g.g.Url(c.Issue),
				LastModified: c.Updated,
				Kind:         source.Gerrit,
			}
		}
	}()

	return ret
}

// See source.Source.
func (g *gerritSource) ByUser(string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	close(ret)
	return ret
}
