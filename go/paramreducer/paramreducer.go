package paramreducer

/*
  paramreducer produces a new ParamSet from a ParamSet and a Query against that ParamSet.

	Given a corpus of traces that have ids of:

		config=8888,cpu=x86,res=ms
		config=565,cpu=x86,res=count
		config=565,cpu=arm,res=cov
		config=gles,cpu=arm,res=bytes

	And a query against that corpus:

    config=565,res=[cov,bytes]

	We want to return a paramset that represents all the legal options for queries
	against the subset of traces that match the query:

		cpu=[arm]
		config=[565, gles]
		res=[count, cov]

	I.e. our query matches the corpus as follows:

				 config=8888,cpu=x86,res=ms
		**   config=565,cpu=x86,res=count
		** * config=565,cpu=arm,res=cov
			 * config=gles,cpu=arm,res=bytes

		** = matches config=565       => res=[count, cov],   cpu=[arm, x86]
		 * = matches res=[cov, bytes] => config=[565, gles], cpu=[arm]
*/
import (
	"fmt"
	"net/url"
	"sort"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
	"go.skia.org/infra/go/util"
)

type SubQuery struct {
	q  *query.Query
	ps paramtools.ParamSet
}

func (s *SubQuery) Add(key string) {
	if s.q.Matches(key) {
		s.ps.AddParamsFromKey(key)
	}
}

type Reducer struct {
	full       paramtools.ParamSet
	subQueries map[string]*SubQuery
}

// New returns a new Reducer for the given query and full paramset.
func New(q url.Values, full paramtools.ParamSet) (*Reducer, error) {
	// Break apart the query into sub-queries, one for each key present in the query.
	subQueries := map[string]*SubQuery{}
	for key, values := range q {
		subQuery, err := query.New(url.Values{
			key: values,
		})
		if err != nil {
			return nil, fmt.Errorf("Invalid query: %s", err)
		}
		subQueries[key] = &SubQuery{
			q:  subQuery,
			ps: paramtools.ParamSet{},
		}
	}
	return &Reducer{
		full:       full,
		subQueries: subQueries,
	}, nil
}

// Add a structured key from the full corpus.
func (r *Reducer) Add(key string) {
	for _, sub := range r.subQueries {
		sub.Add(key)
	}
}

// Reduce all the data to a final ParamSet that represents
// all the valid options left in full ParamSet.
func (r *Reducer) Reduce() paramtools.ParamSet {
	ret := paramtools.ParamSet{}
	for full_k, full_v := range r.full {
		ss := util.NewStringSet(full_v)
		for k, sub := range r.subQueries {
			if k != full_k {
				ss = ss.Intersect(util.NewStringSet(sub.ps[full_k]))
			}
		}
		values := ss.Keys()
		sort.Strings(values)
		if len(values) > 0 {
			ret[full_k] = values
		}
	}
	return ret
}
