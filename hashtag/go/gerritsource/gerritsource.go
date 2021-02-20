package gerritsource

import (
	"context"
	"fmt"
	"strings"

	"github.com/spf13/viper"
	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/gerrit"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/hashtag/go/source"
)

const (
	searchLimit = 5000
)

// SearchType is the type of search the gerritSource instance will do.
type SearchType string

const (
	Merged   SearchType = "merged"   // Returns only merged commits for user queries.
	Reviewed SearchType = "reviewed" // Returns only reviewed CLs for user queries.
)

// gerritSource implements source.Source.
type gerritSource struct {
	g     *gerrit.Gerrit
	st    SearchType
	terms []*gerrit.SearchTerm
}

// New returns a new Source.
func New(local bool, st SearchType) (source.Source, error) {
	ts, err := auth.NewDefaultTokenSource(local, auth.SCOPE_GERRIT)
	if err != nil {
		return nil, err
	}
	c := httputils.DefaultClientConfig().WithTokenSource(ts).With2xxOnly().Client()
	g, err := gerrit.NewGerrit(viper.GetString("sources.gerrit.url"), c)
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
		st:    st,
		terms: terms,
	}, err
}

// toTerms converts a source.Query into gerrit.SearchTerms.
func (g *gerritSource) toTerms(q source.Query) []*gerrit.SearchTerm {
	ret := []*gerrit.SearchTerm{}
	ret = append(ret, g.terms...)
	if q.Type == source.HashtagQuery {
		ret = append(ret, &gerrit.SearchTerm{
			Key:   "message",
			Value: q.Value,
		})
	} else if q.Type == source.UserQuery {
		if g.st == Merged {
			ret = append(ret,
				gerrit.SearchOwner(q.Value),
				gerrit.SearchStatus(gerrit.ChangeStatusMerged))
		} else /* Reviewed */ {
			// Omit roller generated CLs.
			ret = append(ret, &gerrit.SearchTerm{
				Key:   "-owner",
				Value: "skia",
			})
			ret = append(ret, &gerrit.SearchTerm{
				Key:   "-owner",
				Value: q.Value,
			})
			ret = append(ret, &gerrit.SearchTerm{
				Key:   "reviewer",
				Value: q.Value,
			})
		}
	}
	if !q.Begin.IsZero() {
		ret = append(ret, &gerrit.SearchTerm{
			Key:   "after",
			Value: q.Begin.Format("2006-01-02"),
		})
	}
	if !q.End.IsZero() {
		ret = append(ret, &gerrit.SearchTerm{
			Key:   "before",
			Value: q.End.Format("2006-01-02"),
		})
	}

	return ret
}

type changeInfoSlice []*gerrit.ChangeInfo

func (p changeInfoSlice) Len() int { return len(p) }
func (p changeInfoSlice) Less(i, j int) bool {
	if p[i].Status != p[j].Status {
		return p[i].Status < p[j].Status
	}
	return p[i].Updated.After(p[j].Updated)
}
func (p changeInfoSlice) Swap(i, j int) { p[i], p[j] = p[j], p[i] }

// See source.Source.
func (g *gerritSource) Search(ctx context.Context, q source.Query) <-chan source.Artifact {
	ret := make(chan source.Artifact)
	go func() {
		defer close(ret)
		changes, err := g.g.Search(context.Background(), searchLimit, false, g.toTerms(q)...)
		if err != nil {
			sklog.Errorf("Failed to build Gerrit search: %s", err)
			return
		}
		for _, c := range changes {
			ret <- source.Artifact{
				// TODO(jcgregorio) - Make Title formatted HTML to allow finer grained control.
				Title:        fmt.Sprintf("%d/%d - %s - %s", c.Insertions, c.Deletions, c.Subject, c.Status),
				URL:          g.g.Url(c.Issue),
				LastModified: c.Updated,
			}
		}
	}()

	return ret
}
