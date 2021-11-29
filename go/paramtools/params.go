// Package paramtools provides Params and ParamSet.
package paramtools

import (
	"sort"
	"strings"

	"go.skia.org/infra/go/sets"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

// Params is a set of key,value pairs.
type Params map[string]string

// ParamSet is a set of keys and the possible values that the keys could have. I.e.
// the []string should contain no duplicates.
type ParamSet map[string][]string

// ReadOnlyParamSet is a ParamSet that doesn't allow offer any mutating methods.
//
// Note that you can still modify the map, but hopefully the name along with the
// removal of the mutating methods will catch most of the mis-uses.
type ReadOnlyParamSet map[string][]string

// NewParams returns the parsed structured key (see query) as Params.
//
// It presumes a valid key, i.e. something that passed query.ValidateKey.
func NewParams(key string) Params {
	ret := Params{}
	parts := strings.Split(key, ",")
	parts = parts[1 : len(parts)-1]
	for _, s := range parts {
		pair := strings.SplitN(s, "=", 2)
		ret[pair[0]] = pair[1]
	}
	return ret
}

// Add adds each set of Params in order to this Params.
//
// Values in p will be overwritten.
func (p Params) Add(b ...Params) {
	for _, oneMap := range b {
		for k, v := range oneMap {
			p[k] = v
		}
	}
}

// Copy returns a copy of the Params.
func (p Params) Copy() Params {
	ret := make(Params, len(p))
	for k, v := range p {
		ret[k] = v
	}
	return ret
}

// Equal returns true if this Params equals a.
func (p Params) Equal(a Params) bool {
	if len(a) != len(p) {
		return false
	}
	// Since they are the same size we only need to check from one side, i.e.
	// compare a's values to p's values.
	for k, v := range a {
		if bv, ok := p[k]; !ok || bv != v {
			return false
		}
	}
	return true
}

// Keys returns the keys of the Params.
func (p Params) Keys() []string {
	ret := make([]string, 0, len(p))
	for v := range p {
		ret = append(ret, v)
	}

	return ret
}

// NewParamSet returns a new ParamSet initialized with the given maps of parameters.
func NewParamSet(ps ...Params) ParamSet {
	ret := ParamSet{}
	for _, onePS := range ps {
		ret.AddParams(onePS)
	}
	return ret
}

// NewReadOnlyParamSet returns a new ReadOnlyParamSet initialized with the given
// maps of parameters.
func NewReadOnlyParamSet(ps ...Params) ReadOnlyParamSet {
	return ReadOnlyParamSet(NewParamSet(ps...))
}

// AddParams adds the Params to this ParamSet.
func (p ParamSet) AddParams(ps Params) {
	for k, v := range ps {
		// You might be tempted to replace this with
		// sort.SearchStrings(), but that's actually slower for short
		// slices. The breakpoint seems to around 50, and since most
		// of our ParamSet lists are short that ends up being slower.
		params := p[k]
		if !util.In(v, params) {
			p[k] = append(params, v)
		}
	}
}

// AddParamsFromKey is the same as calling
//
//   paramset.AddParams(NewParams(key))
//
// but without creating the intermediate Params.
//
// It presumes a valid key, i.e. something that passed query.ValidateKey.
func (p ParamSet) AddParamsFromKey(key string) {
	parts := strings.Split(key, ",")
	parts = parts[1 : len(parts)-1]
	for _, s := range parts {
		pair := strings.SplitN(s, "=", 2)
		params := p[pair[0]]
		if !util.In(pair[1], params) {
			p[pair[0]] = append(params, pair[1])
		}
	}
}

// AddParamSet adds the ParamSet or ReadOnlyParamSet to this ParamSet.
func (p ParamSet) AddParamSet(ps map[string][]string) {
	for k, arr := range ps {
		if _, ok := p[k]; !ok {
			p[k] = append([]string{}, arr...)
		} else {
			for _, v := range arr {
				if !util.In(v, p[k]) {
					p[k] = append(p[k], v)
				}
			}
		}
	}
}

// Equal returns true if the given Paramset contain exactly the same keys and associated
// values as this one. Side Effect: both ParamSets will be normalized after this call (their
// values will be sorted) if they have the same number of keys.
func (p ParamSet) Equal(right map[string][]string) bool {
	if len(p) != len(right) {
		return false
	}
	p.Normalize()
	ParamSet(right).Normalize()
	for k, leftValues := range p {
		rightValues, ok := right[k]
		if !ok {
			return false
		}
		// Due to normalize, we expect leftValues and rightValues to be in sorted order
		if len(leftValues) != len(rightValues) {
			return false
		}
		for i := range leftValues {
			if leftValues[i] != rightValues[i] {
				return false
			}
		}
	}
	return true
}

// Keys returns the keys of the ReadOnlyParamSet.
func (p ReadOnlyParamSet) Keys() []string {
	ret := make([]string, 0, len(p))
	for v := range p {
		ret = append(ret, v)
	}

	return ret
}

// Keys returns the keys of the ParamSet.
func (p ParamSet) Keys() []string {
	return ReadOnlyParamSet(p).Keys()
}

// Copy returns a copy of the ParamSet.
func (p ParamSet) Copy() ParamSet {
	ret := ParamSet{}
	for k, v := range p {
		newV := make([]string, len(v), len(v))
		copy(newV, v)
		ret[k] = newV
	}

	return ret
}

// FrozenCopy returns a copy of the ParamSet as a ReadOnlyParamSet.
func (p ParamSet) FrozenCopy() ReadOnlyParamSet {
	return ReadOnlyParamSet(p.Copy())
}

// Freeze returns the ReadOnlyParamSet version of the ParamSet.
//
// It is up to the caller to make sure the original ParamSet is not modified, or
// call FrozenCopy() instead.
func (p ParamSet) Freeze() ReadOnlyParamSet {
	return ReadOnlyParamSet(p)
}

// Normalize all the values by sorting them.
func (p ParamSet) Normalize() {
	for _, arr := range p {
		sort.Strings(arr)
	}
}

// Matches returns true if the params in 'p' match the sets given in 'right'.
// For every key in 'p' there has to be a matching key in 'right' and the
// intersection of their values must be not empty.
func (p ReadOnlyParamSet) Matches(right ReadOnlyParamSet) bool {
	for key, vals := range p {
		rightVals, ok := right[key]
		if !ok {
			return false
		}

		found := false
		for _, targetVal := range vals {
			if util.In(targetVal, rightVals) {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// Matches returns true if the params in 'p' match the sets given in 'right'.
// For every key in 'p' there has to be a matching key in 'right' and the
// intersection of their values must be not empty.
func (p ParamSet) Matches(right ParamSet) bool {
	return ReadOnlyParamSet(p).Matches(ReadOnlyParamSet(right))
}

// CartesianProduct returns a channel of Params that represent the Cartesian
// Product of all the values for the given keys.
func (p ParamSet) CartesianProduct(keys []string) (<-chan Params, error) {
	ret := make(chan Params)
	counts := make([]int, len(keys))
	for i, key := range keys {
		counts[i] = len(p[key])
	}
	cpChan, err := sets.CartesianProduct(counts)
	if err != nil {
		close(ret)
		return nil, skerr.Wrapf(err, "can not make cartesian product")
	}

	go func() {
		for indices := range cpChan {
			v := Params{}
			for i, n := range indices {
				v[keys[i]] = p[keys[i]][n]
			}
			ret <- v
		}
		close(ret)
	}()

	return ret, nil
}

// MatchesParams returns true if the params in 'p' match the values given in
// 'right'. For every key in 'p' there has to be a matching key in 'right' and
// the intersection of their values must be not empty.
func (p ReadOnlyParamSet) MatchesParams(right Params) bool {
	for key, vals := range p {
		rightVal, ok := right[key]
		if !ok {
			return false
		}

		found := false
		for _, targetVal := range vals {
			if targetVal == rightVal {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// MatchesParams returns true if the params in 'p' match the values given in
// 'right'. For every key in 'p' there has to be a matching key in 'right' and
// the intersection of their values must be not empty.
func (p ParamSet) MatchesParams(right Params) bool {
	return ReadOnlyParamSet(p).MatchesParams(right)
}

// Size returns the total number of values in the ReadOnlyParamSet.
func (p ReadOnlyParamSet) Size() int {
	var ret int
	for _, vals := range p {
		ret += len(vals)
	}
	return ret
}

// Size returns the total number of values in the ParamSet.
func (p ParamSet) Size() int {
	return ReadOnlyParamSet(p).Size()
}

// ParamMatcher is a list of Paramsets that can be matched against. The primary
// purpose is to match against a set of rules, e.g. ignore rules.
type ParamMatcher []ParamSet

// MatchAny returns true if the given ParamSet matches any of the rules in the matcher.
func (p ParamMatcher) MatchAny(params ParamSet) bool {
	for _, oneRule := range p {
		if oneRule.Matches(params) {
			return true
		}
	}
	return false
}

// MatchAnyParams returns true if the given Params matches any of the rules in the matcher.
func (p ParamMatcher) MatchAnyParams(params Params) bool {
	for _, oneRule := range p {
		if oneRule.MatchesParams(params) {
			return true
		}
	}
	return false
}
