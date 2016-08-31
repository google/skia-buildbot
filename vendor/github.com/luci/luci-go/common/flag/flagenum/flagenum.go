// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package flagenum is a utility package which facilitates implementation of
// flag.Value, json.Marshaler, and json.Unmarshaler interfaces via a string-to-
// value mapping.
package flagenum

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strings"
)

// Enum is a mapping of enumeration key strings to values that can be used as
// flags.
//
// Strings can be mapped to any value type that is comparable via
// reflect.DeepEqual.
type Enum map[string]interface{}

// GetKey performs reverse lookup of the enumeration value, returning the
// key that corresponds to the value.
//
// If multiple keys correspond to the same value, the result is undefined.
func (e Enum) GetKey(value interface{}) string {
	for k, v := range e {
		if reflect.DeepEqual(v, value) {
			return k
		}
	}
	return ""
}

// GetValue returns the mapped enumeration value associated with a key.
func (e Enum) GetValue(key string) (interface{}, error) {
	for k, v := range e {
		if k == key {
			return v, nil
		}
	}
	return nil, fmt.Errorf("flagenum: Invalid value; must be one of [%s]", e.Choices())
}

// Choices returns a comma-separated string listing sorted enumeration choices.
func (e Enum) Choices() string {
	keys := make([]string, 0, len(e))
	for k := range e {
		if k == "" {
			k = `""`
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ", ")
}

// Sets the value v to the enumeration value mapped to the supplied key.
func (e Enum) setValue(v interface{}, key string) error {
	i, err := e.GetValue(key)
	if err != nil {
		return err
	}

	vValue := reflect.ValueOf(v).Elem()
	if !vValue.CanSet() {
		panic(fmt.Errorf("flagenum: Cannot set supplied value, %v", vValue))
	}

	iValue := reflect.ValueOf(i)
	if !vValue.Type().AssignableTo(iValue.Type()) {
		panic(fmt.Errorf("flagenum: Enumeration type (%v) is incompatible with supplied value (%v)",
			vValue.Type(), iValue.Type()))
	}

	vValue.Set(iValue)
	return nil
}

// FlagSet implements flag.Value's Set semantics. It identifies the mapped value
// associated with the supplied key and stores it in the supplied interface.
//
// The interface, v, must be a valid pointer to the mapped enumeration type.
func (e Enum) FlagSet(v interface{}, key string) error {
	return e.setValue(v, key)
}

// FlagString implements flag.Value's String semantics.
func (e Enum) FlagString(v interface{}) string {
	return e.GetKey(v)
}

// JSONUnmarshal implements json.Unmarshaler's UnmarshalJSON semantics. It
// parses data containing a quoted string, identifies the enumeration value
// associated with that string, and stores it in the supplied interface.
//
// The interface, v, must be a valid pointer to the mapped enumeration type.
// a string corresponding to one of the enum's keys.
func (e Enum) JSONUnmarshal(v interface{}, data []byte) error {
	s := ""
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	return e.FlagSet(v, s)
}

// JSONMarshal implements json.Marshaler's MarshalJSON semantics. It marshals
// the value in the supplied interface to its associated key and emits a quoted
// string containing that key.
//
// The interface, v, must be a valid pointer to the mapped enumeration type.
func (e Enum) JSONMarshal(v interface{}) ([]byte, error) {
	key := e.GetKey(v)
	data, err := json.Marshal(&key)
	if err != nil {
		return nil, err
	}
	return data, nil
}
