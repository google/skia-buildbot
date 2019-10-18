package codesearchsource

import (
	"context"

	"go.skia.org/infra/go/codesearch"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
)

type codesearchSource struct {
	cs *codesearch.CodeSearch
}

// New returns a new Source.
func New() source.Source {
	c := httputils.DefaultClientConfig().With2xxOnly().Client()
	cs := codesearch.New(c)
	return &codesearchSource{
		cs: cs,
	}
}

func (cs *codesearchSource) ByHashtag(hashtag string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)
		results, err := cs.cs.Query(context.Background(), codesearch.SkiaBaseQuery+" "+hashtag, nil)
		if err != nil {
			sklog.Errorf("Failed to build code search: %s", err)
			return
		}
		for _, r := range results.SearchResult {
			ret <- source.Artifact{
				Title: r.TopFile.File.Name,
				URL:   cs.cs.URL(r),
				Kind:  source.CodeSearch,
			}
		}
	}()

	return ret

}

func (cs *codesearchSource) ByUser(hashtag string) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	close(ret)
	return ret
}
