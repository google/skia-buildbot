package gerritsource

import (
	"context"

	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
)

type gerritSource struct {
	g *gerrit.Gerrit
}

// New returns a new *Source.
func New() (source.Source, error) {
	c := httputils.DefaultClientConfig().With2xxOnly().Client()
	g, err := gerrit.NewGerrit(gerrit.GERRIT_SKIA_URL, "", c)
	return &gerritSource{
		g: g,
	}, err
}

func (g *gerritSource) ByHashtag(hashtag string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)
		term := &gerrit.SearchTerm{
			Key:   "message",
			Value: hashtag,
		}
		changes, err := g.g.Search(context.Background(), gerrit.MAX_GERRIT_LIMIT, term)
		if err != nil {
			sklog.Errorf("Failed to build Gerrit search: %s", err)
			close(ret)
			return
		}
		for _, c := range changes {
			ret <- source.Artifact{
				Title: c.Subject,
			}
		}
	}()

	return ret
}
func (g *gerritSource) ByUser(string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	close(ret)
	return ret
}

// Validate that the concrete liveness faithfully implements the Liveness interface.
var _ source.Source = (*gerritSource)(nil)
