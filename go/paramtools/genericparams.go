package paramtools

import (
	"strings"

	"github.com/cheekybits/genny/generic"
)

type Key string
type Value string
type EncodedKey string
type EncodedValue string

type GenericKey generic.Type
type GenericValue generic.Type

type GenericParams map[GenericKey]GenericValue

// NewGenericParams returns the parsed structured key (see query) as GenericParams.
//
// It presumes a valid key, i.e. something that passed query.ValidateKey.
func NewGenericParams(key GenericKey) GenericParams {
	ret := GenericParams{}
	parts := strings.Split(key.(string), ",")
	parts = parts[1 : len(parts)-1]
	for _, s := range parts {
		pair := strings.SplitN(s, "=", 2)
		ret[pair[0]] = pair[1]
	}
	return ret
}

// Add adds each set of GenericParams in order to this GenericParams.
//
// Values in p will be overwritten.
func (p GenericParams) Add(b ...GenericParams) {
	for _, oneMap := range b {
		for k, v := range oneMap {
			p[k] = v
		}
	}
}

// Copy returns a copy of the GenericParams.
func (p GenericParams) Copy() GenericParams {
	ret := make(GenericParams, len(p))
	for k, v := range p {
		ret[k] = v
	}
	return ret
}

// Equal returns true if this GenericParams equals a.
func (p GenericParams) Equal(a GenericParams) bool {
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

// Keys returns the keys of the GenericParams.
func (p GenericParams) Keys() []GenericKey {
	ret := make([]GenericKey, 0, len(p))
	for v := range p {
		ret = append(ret, v)
	}

	return ret
}
