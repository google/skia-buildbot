package preflightqueryprocessor

import (
	"sync"

	"go.skia.org/infra/go/paramtools"
	"go.skia.org/infra/go/query"
)

// A base type, containing query and shared mutex + paramset.
// This mutex is also shared for managing count variable.
type preflightQueryBaseProcessor struct {
	q              *query.Query
	sharedMux      *sync.Mutex
	sharedParamSet *paramtools.ParamSet
}

func (p *preflightQueryBaseProcessor) AddParams(ps paramtools.Params) {
	p.sharedMux.Lock()
	defer p.sharedMux.Unlock()
	p.sharedParamSet.AddParams(ps)
}

func (p *preflightQueryBaseProcessor) GetQuery() *query.Query {
	return p.q
}

func (p *preflightQueryBaseProcessor) SetReferenceParamKey(key string, referenceParamSet paramtools.ReadOnlyParamSet) {
	p.sharedMux.Lock()
	defer p.sharedMux.Unlock()
	(*p.sharedParamSet)[key] = referenceParamSet[key]
}
