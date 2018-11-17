package util

import (
	"fmt"
	"strings"
)

// StringSet is a set of strings, represented by the keys of a map.
type StringSet map[string]bool

// NewStringSet returns the given list(s) of strings as a StringSet.
func NewStringSet(lists ...[]string) StringSet {
	ret := make(map[string]bool)
	for _, list := range lists {
		for _, entry := range list {
			ret[entry] = true
		}
	}
	return ret
}

// Copy returns a copy of the StringSet such that reflect.DeepEqual returns true
// for the original and copy. In particular, preserves nil input.
func (s StringSet) Copy() StringSet {
	if s == nil {
		return nil
	}
	ret := make(StringSet, len(s))
	for k, v := range s {
		ret[k] = v
	}
	return ret
}

// Keys returns the keys of a StringSet
func (s StringSet) Keys() []string {
	ret := make([]string, 0, len(s))
	for v := range s {
		ret = append(ret, v)
	}
	return ret
}

// String returns a comma seperated list of the values of the set.
func (s StringSet) String() string {
	if s == nil {
		return "<nil StringSet>"
	}
	return fmt.Sprintf("[%s]", strings.Join(s.Keys(), ","))
}

// Intersect returns a new StringSet containing all the original strings that are also in
// StringSet other
func (s StringSet) Intersect(other StringSet) StringSet {
	resultSet := make(StringSet, len(s))
	for val := range s {
		if other[val] {
			resultSet[val] = true
		}
	}

	return resultSet
}

// Complement returns a new StringSet containing all the original strings that are not in
// StringSet other
func (s StringSet) Complement(other StringSet) StringSet {
	resultSet := make(StringSet, len(s))
	for val := range s {
		if !other[val] {
			resultSet[val] = true
		}
	}

	return resultSet
}

// Union returns a new StringSet containing all the original strings and all the strings in
// StringSet other
func (s StringSet) Union(other StringSet) StringSet {
	resultSet := make(StringSet, len(s))
	for val := range s {
		resultSet[val] = true
	}

	for val := range other {
		resultSet[val] = true
	}

	return resultSet
}

func (s StringSet) Equals(other StringSet) bool {
	if len(s) != len(other) {
		return false
	}
	for val := range s {
		if !other[val] {
			return false
		}
	}
	return true
}

// AddLists adds lists of strings to the StringSet and returns
// the receiving StringSet.
func (s StringSet) AddLists(lists ...[]string) StringSet {
	for _, oneList := range lists {
		for _, item := range oneList {
			s[item] = true
		}
	}
	return s
}
