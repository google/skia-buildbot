// Package params provides Params and ParamSet.
package paramtools

import (
	"bytes"
	"compress/gzip"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"go.skia.org/infra/go/util"
)

// Params is a set of key,value pairs.
type Params map[string]string

// ParamSet is a set of keys and the possible values that the keys could have. I.e.
// the []string should contain no duplicates.
type ParamSet map[string][]string

// NewParams returns the parsed structured key (see query) as Params.
//
// It presumes a valid key, i.e. something that passed query.ValidateKey.
func NewParams(key string) Params {
	ret := Params{}
	parts := strings.Split(key, ",")
	parts = parts[1 : len(parts)-1]
	for _, s := range parts {
		pair := strings.Split(s, "=")
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

// Dup returns a copy of the Params.
func (p Params) Dup() Params {
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

// Add the Params to this ParamSet.
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
// but without creating the intermedite Params.
//
// It presumes a valid key, i.e. something that passed query.ValidateKey.
func (p ParamSet) AddParamsFromKey(key string) {
	parts := strings.Split(key, ",")
	parts = parts[1 : len(parts)-1]
	for _, s := range parts {
		pair := strings.Split(s, "=")
		params := p[pair[0]]
		if !util.In(pair[1], params) {
			p[pair[0]] = append(params, pair[1])
		}
	}
}

// Add the ParamSet to this ParamSet.
func (p ParamSet) AddParamSet(ps ParamSet) {
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

// Keys returns the keys of the ParamSet.
func (p ParamSet) Keys() []string {
	ret := make([]string, 0, len(p))
	for v := range p {
		ret = append(ret, v)
	}

	return ret
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

// Normalize all the values by sorting them.
func (p ParamSet) Normalize() {
	for _, arr := range p {
		sort.Strings(arr)
	}
}

// Matches returns true if the params in 'p' match the sets given in 'right'.
// For every key in 'p' there has to be a matching key in 'right' and
// the intersection of their values must be not empty.
func (p ParamSet) Matches(right ParamSet) bool {
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

// ParamMatcher is a list of Paramsets that can be matched against. The primary
// purpose is to match against a set of rules, e.g. ignore rules.
type ParamMatcher []ParamSet

// MatchAny returns true if the given Paramset matches any of the rules in the matcher.
func (p ParamMatcher) MatchAny(params ParamSet) bool {
	for _, oneRule := range p {
		if oneRule.Matches(params) {
			return true
		}
	}
	return false
}

type OrderedParamSet struct {
	KeyOrder      []string
	ParamSet      ParamSet
	paramsEncoder *paramsEncoder
}

func NewOrderedParamSet() *OrderedParamSet {
	return &OrderedParamSet{
		KeyOrder:      []string{},
		ParamSet:      ParamSet{},
		paramsEncoder: nil,
	}
}

// Returns all the keys and their values that don't exist in the OrderedParamSet.
func (o *OrderedParamSet) Check(p ParamSet) ParamSet {
	ret := ParamSet{}
	for k, newValues := range p {
		if values, ok := o.ParamSet[k]; !ok {
			ret[k] = newValues
		} else {
			for _, v := range newValues {
				if !util.In(v, values) {
					ret[k] = append(ret[k], v)
				}
			}
		}
	}
	return ret
}

func (o *OrderedParamSet) Update(p ParamSet) {
	o.paramsEncoder = nil
	// Add new keys to KeyOrder.
	// Append new values if they don't exist.
	for k, values := range p {
		if !util.In(k, o.KeyOrder) {
			o.KeyOrder = append(o.KeyOrder, k)
			o.ParamSet[k] = []string{}
		}
		currentValues := o.ParamSet[k]
		for _, v := range values {
			if !util.In(v, currentValues) {
				currentValues = append(currentValues, v)
			}
		}
		o.ParamSet[k] = currentValues
	}
}

func (o *OrderedParamSet) Encode() ([]byte, error) {
	var b bytes.Buffer
	gz := gzip.NewWriter(&b)
	enc := json.NewEncoder(gz)
	if err := enc.Encode(o); err != nil {
		return nil, fmt.Errorf("Failed to encode: %s", err)
	}
	if err := gz.Close(); err != nil {
		return nil, fmt.Errorf("Failed to gzip: %s", err)
	}
	return b.Bytes(), nil
}

func NewOrderedParamSetFromBytes(b []byte) (*OrderedParamSet, error) {
	buf := bytes.NewBuffer(b)
	gz, err := gzip.NewReader(buf)
	if err != nil {
		return nil, fmt.Errorf("Failed to create reader: %s", err)
	}
	dec := json.NewDecoder(gz)
	p := &OrderedParamSet{}
	if err := dec.Decode(&p); err != nil {
		return nil, fmt.Errorf("Failed to decode: %s", err)
	}
	return p, nil
}

func (o *OrderedParamSet) buildEncoder() *paramsEncoder {
	ret := &paramsEncoder{
		keys:   map[string]int{},
		values: make([]map[string]int, len(o.KeyOrder)),
	}
	for keyIndex, key := range o.KeyOrder {
		ret.keys[key] = keyIndex
		valueMap := map[string]int{}
		for valueIndex, value := range o.ParamSet[key] {
			valueMap[value] = valueIndex
		}
		ret.values[keyIndex] = valueMap
	}
	return ret
}

type paramsEncoder struct {
	keys   map[string]int
	values []map[string]int
}

func (p *paramsEncoder) encode(params Params) ([]byte, error) {
	ints := make([]int64, 0, len(params)*2)
	for key, value := range params {
		keyIndex, ok := p.keys[key]
		if !ok {
			return nil, fmt.Errorf("Unknown key.")
		}
		valueIndex, ok := p.values[keyIndex][value]
		if !ok {
			return nil, fmt.Errorf("Unknown value.")
		}
		ints = append(ints, int64(keyIndex), int64(valueIndex))
	}
	buf := []byte{}
	perIntBuf := make([]byte, binary.MaxVarintLen64)
	for _, i := range ints {
		n := binary.PutVarint(perIntBuf, i)
		buf = append(buf, perIntBuf[:n]...)
	}

	return buf, nil
}

func (o *OrderedParamSet) EncodeParams(p Params) ([]byte, error) {
	if o.paramsEncoder == nil {
		o.paramsEncoder = o.buildEncoder()
	}
	return o.paramsEncoder.encode(p)
}

func (o *OrderedParamSet) DecodeParams(b []byte) (Params, error) {
	i64, n := binary.Varint(b)
	ints := []int64{}
	for n > 0 {
		ints = append(ints, i64)
		b = b[n:]
		i64, n = binary.Varint(b)
	}
	if n < 0 {
		return nil, fmt.Errorf("Failed to decode bytes")
	}
	ret := Params{}
	for i := 0; i < len(ints); i += 2 {
		keyIndex := ints[i]
		if keyIndex > int64(len(o.KeyOrder)) || keyIndex < 0 {
			return nil, fmt.Errorf("Got invalid key index: %d", keyIndex)
		}
		key := o.KeyOrder[keyIndex]
		values := o.ParamSet[key]
		valueIndex := ints[i+1]
		if valueIndex > int64(len(values)) || valueIndex < 0 {
			return nil, fmt.Errorf("Got invalid value index for %s: %d", key, valueIndex)
		}
		value := values[valueIndex]
		ret[key] = value
	}
	return ret, nil
}
