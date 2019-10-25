package gerritsource

import (
	"context"
	"strings"

	"github.com/spf13/viper"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
)

// gerritSource implements source.Source.
type gerritSource struct {
	g     *gerrit.Gerrit
	terms []*gerrit.SearchTerm
}

// New returns a new Source.
func New() (source.Source, error) {
	c := httputils.DefaultClientConfig().With2xxOnly().Client()
	g, err := gerrit.NewGerrit(viper.GetString("sources.gerrit.url"), "", c)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to create gerrit instance")
	}
	terms := []*gerrit.SearchTerm{}
	for _, filter := range viper.GetStringSlice("sources.gerrit.filters") {
		parts := strings.SplitN(filter, ":", 2)
		if len(parts) != 2 {
			return nil, skerr.Fmt("All Gerrit filters must start with an operator, e.g. 'owner:'.")
		}
		terms = append(terms, &gerrit.SearchTerm{
			Key:   parts[0],
			Value: parts[1],
		})
	}
	return &gerritSource{
		g:     g,
		terms: terms,
	}, err
}

// See source.Source.
func (g *gerritSource) ByHashtag(hashtag string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)
		terms := []*gerrit.SearchTerm{
			{
				Key:   "message",
				Value: hashtag,
			},
		}
		terms = append(terms, g.terms...)
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
