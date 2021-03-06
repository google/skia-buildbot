package codesearchsource

import (
	"context"

	"github.com/spf13/viper"
	"go.skia.org/infra/go/codesearch"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
)

// codesearchSource implements source.Source.
type codesearchSource struct {
	cs *codesearch.CodeSearch

	// prefix is added to each query.
	prefix string
}

// New returns a new Source.
func New() (source.Source, error) {
	c := httputils.DefaultClientConfig().With2xxOnly().Client()
	cs := codesearch.New(c)
	return &codesearchSource{
		cs:     cs,
		prefix: viper.GetString("sources.codesearch.prefix"),
	}, nil
}

// toString converts a source.Query into a search string for the Code Search API.
func (cs *codesearchSource) toString(q source.Query) string {
	// codesearch has no concept of time, so we ignore q.Begin and q.End.
	// It also has no concept of authorship, so we ignore q.Type.
	return cs.prefix + " " + q.Value
}

// See source.Source.
func (cs *codesearchSource) Search(ctx context.Context, q source.Query) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)
		results, err := cs.cs.Query(ctx, cs.toString(q), nil)
		if err != nil {
			sklog.Errorf("Failed to build code search: %s", err)
			return
		}
		for _, r := range results.SearchResult {
			ret <- source.Artifact{
				Title: r.TopFile.File.Name,
				URL:   cs.cs.URL(r),
			}
		}
	}()

	return ret

}
