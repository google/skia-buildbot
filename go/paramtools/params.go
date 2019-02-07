// Package params provides Params and ParamSet.
package paramtools

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"sync"

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

// OrderedParamSet is a ParamSet that keeps track of the order in which
// keys and values were added, which allows for compressing Params
// that make up the ParamSet to a smaller number of bytes.
//
// For example, if the paramset has just one key and two values:
//
//    "config": ["8888", "565"]
//
// Then a Params of
//
//    "config": "565"
//
// can be represented as two integers, the offset of "config" into the list of
// ordered keys (0), and the offset of "565" into the list of all values seen
// for that key (1).
//
// As a structured key that could be represented as:
//
//  ,0=1,
//
// Which is a savings over
//
//  ,config=565,
//
// OrderedParamSet is not Go routine safe.
type OrderedParamSet struct {
	mutex         sync.Mutex
	KeyOrder      []string
	ParamSet      ParamSet
	paramsEncoder *paramsEncoder
	paramsDecoder *paramsDecoder
}

func NewOrderedParamSet() *OrderedParamSet {
	return &OrderedParamSet{
		KeyOrder:      []string{},
		ParamSet:      ParamSet{},
		paramsEncoder: nil,
		paramsDecoder: nil,
	}
}

func (o *OrderedParamSet) Copy() *OrderedParamSet {
	ret := &OrderedParamSet{
		KeyOrder: make([]string, len(o.KeyOrder)),
		ParamSet: o.ParamSet.Copy(),
	}
	copy(ret.KeyOrder, o.KeyOrder)
	return ret
}

// Delta returns all the keys and their values that don't exist in the
// OrderedParamSet.
func (o *OrderedParamSet) Delta(p ParamSet) ParamSet {
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

// Update adds all the key/value pairs from ParamSet, which is usually the
// ParamSet returned from Delta().
func (o *OrderedParamSet) Update(p ParamSet) {
	o.mutex.Lock()
	defer o.mutex.Unlock()
	o.paramsEncoder = nil
	o.paramsDecoder = nil
	// Add new keys to KeyOrder.
	// Append new values if they don't exist.
	for k, values := range p {
		currentValues, ok := o.ParamSet[k]
		if !ok {
			o.KeyOrder = append(o.KeyOrder, k)
			o.ParamSet[k] = []string{}
			currentValues = []string{}
		}
		for _, v := range values {
			if !util.In(v, currentValues) {
				currentValues = append(currentValues, v)
			}
		}
		o.ParamSet[k] = currentValues
	}
}

// Encode the OrderedParamSet as a byte slice.
//
func (o *OrderedParamSet) Encode() ([]byte, error) {
	var b bytes.Buffer
	err := gob.NewEncoder(&b).Encode(o)
	return b.Bytes(), err
}

// NewOrderedParamSetFromBytes creates an OrderedParamSet using a byte slice
// previously returned from Encode().
//
func NewOrderedParamSetFromBytes(b []byte) (*OrderedParamSet, error) {
	p := NewOrderedParamSet()
	buf := bytes.NewBuffer(b)
	err := gob.NewDecoder(buf).Decode(&p)
	return p, err
}

// buildEncoder builds a paramsEncoder for the current state of the OrderedParamSet.
func (o *OrderedParamSet) buildEncoder() *paramsEncoder {
	ret := &paramsEncoder{
		keyOrder: o.KeyOrder,
		keys:     map[string]string{},
		values:   map[string]map[string]string{},
	}
	for keyIndex, key := range o.KeyOrder {
		keyString := strconv.Itoa(keyIndex)
		ret.keys[key] = keyString
		valueMap := map[string]string{}
		for valueIndex, value := range o.ParamSet[key] {
			valueMap[value] = strconv.Itoa(valueIndex)
		}
		ret.values[keyString] = valueMap
	}
	return ret
}

// buildDecoder builds a paramsDecoder for the current state of the OrderedParamSet.
func (o *OrderedParamSet) buildDecoder() *paramsDecoder {
	ret := &paramsDecoder{
		keys:   map[string]string{},
		values: map[string]map[string]string{},
	}
	for keyIndex, key := range o.KeyOrder {
		keyString := strconv.Itoa(keyIndex)
		ret.keys[keyString] = key
		valueMap := map[string]string{}
		for valueIndex, value := range o.ParamSet[key] {
			valueMap[strconv.Itoa(valueIndex)] = value
		}
		ret.values[keyString] = valueMap
	}
	return ret
}

// paramsDecoder is built from the data in an OrderedParamSet and can efficiently
// decode Params for that OrderedParamSet.
//
// It builds up data structures that allow parsing params by lookup table.
type paramsDecoder struct {
	// keys maps the offset, as a string, to the key name.
	keys map[string]string

	// values maps the key name to a map of value offset, as a string, to the value.
	values map[string]map[string]string
}

// decodeStringToParams takes a string converts it to a Params.
//
// The input string representation is integer pairs as a structured key:
//
//   ,0=1,1=3,3=0,
//
func (p *paramsDecoder) decodeStringToParams(s string) (Params, error) {
	ret := Params{}
	for _, pair := range strings.Split(s, ",") {
		if pair == "" {
			continue
		}
		parts := strings.Split(pair, "=")
		if len(parts) != 2 {
			return nil, fmt.Errorf("Failed to parse: %s", pair)
		}
		key, ok := p.keys[parts[0]]
		if !ok {
			return nil, fmt.Errorf("Failed to find key: %q", parts[0])
		}
		value, ok := p.values[parts[0]][parts[1]]
		if !ok {
			return nil, fmt.Errorf("Failed to find value: %q", parts[1])
		}
		ret[key] = value
	}
	return ret, nil
}

// paramsEncoder is built from the data in an OrderedParamSet and can efficiently
// encode Params for that OrderedParamSet.
//
// It builds up data structures that allow encoding Params by lookup table.
type paramsEncoder struct {
	// keyOrder is the order of the keys.
	keyOrder []string

	// keys maps the key name to its offset in OrdereredParamSet.KeyOrder, the offset being
	// given as a string, i.e. "2".
	keys map[string]string

	// values maps the key key offset to a map that maps the value to its offset, the offset being
	// given as a string, i.e. "2".
	values map[string]map[string]string
}

// encodeAsString takes a Params and finds the indexes of both the key and its value in
// the OrderedParamSet and then writes a string representation.
//
// The representation is integer pairs as a structured key:
//
//   ,0=1,1=3,3=0,
func (p *paramsEncoder) encodeAsString(params Params) (string, error) {
	ret := []string{","}
	for _, key := range p.keyOrder {
		value, ok := params[key]
		if !ok {
			continue
		}
		keyIndex, ok := p.keys[key]
		if !ok {
			return "", fmt.Errorf("Unknown key.")
		}
		valueIndex, ok := p.values[keyIndex][value]
		if !ok {
			return "", fmt.Errorf("Unknown value.")
		}
		ret = append(ret, keyIndex, "=", valueIndex, ",")
	}
	if len(ret) == 1 {
		return "", fmt.Errorf("No params encoded.")
	}
	return strings.Join(ret, ""), nil
}

func (o *OrderedParamSet) getParamsEncoder() *paramsEncoder {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.paramsEncoder == nil {
		o.paramsEncoder = o.buildEncoder()
	}

	return o.paramsEncoder
}

// EncodeParamsAsString encodes the Params as a string.
func (o *OrderedParamSet) EncodeParamsAsString(p Params) (string, error) {
	return o.getParamsEncoder().encodeAsString(p)
}

func (o *OrderedParamSet) getParamsDecoder() *paramsDecoder {
	o.mutex.Lock()
	defer o.mutex.Unlock()

	if o.paramsDecoder == nil {
		o.paramsDecoder = o.buildDecoder()
	}

	return o.paramsDecoder
}

// DecodeParamsFromString decodes the Params from a string.
func (o *OrderedParamSet) DecodeParamsFromString(s string) (Params, error) {
	return o.getParamsDecoder().decodeStringToParams(s)
}
