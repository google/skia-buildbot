// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package templateproto

import (
	"encoding/json"
	"fmt"
	"reflect"
	"sort"
	"strconv"
	"strings"

	"github.com/luci/luci-go/common/data/stringset"
)

// MustNewValue creates a new *Value wrapping v, and panics if v is a bad type
func MustNewValue(v interface{}) *Value {
	ret, err := NewValue(v)
	if err != nil {
		panic(err)
	}
	return ret
}

// NewValue creates a new *Value wrapping v.
//
// Allowed types are:
//   * Any of the explicit *Value_Int - style types
//   * nil -> Null
//   * string -> String
//   * []byte -> Bytes
//   * int, int8, int16, int32, int64 -> Integer
//   * uint, uint8, uint16, uint32, uint64 -> Unsigned
//   * float32, float64 -> Float
//   * bool -> Boolean
//   * map[string]interface{} -> Object
//   * []interface{} -> Array
func NewValue(v interface{}) (*Value, error) {
	switch x := v.(type) {
	case isValue_Value:
		return &Value{x}, nil
	case nil:
		return &Value{&Value_Null{}}, nil
	case int8, int16, int32, int64, int:
		return &Value{&Value_Int{reflect.ValueOf(v).Int()}}, nil
	case uint8, uint16, uint32, uint64, uint:
		return &Value{&Value_Uint{reflect.ValueOf(v).Uint()}}, nil
	case float32, float64:
		return &Value{&Value_Float{reflect.ValueOf(v).Float()}}, nil
	case string:
		return &Value{&Value_Str{x}}, nil
	case []byte:
		return &Value{&Value_Bytes{x}}, nil
	case bool:
		return &Value{&Value_Bool{x}}, nil
	case map[string]interface{}:
		ret, err := json.Marshal(x)
		if err != nil {
			return nil, err
		}
		return &Value{&Value_Object{string(ret)}}, nil
	case []interface{}:
		ret, err := json.Marshal(x)
		if err != nil {
			return nil, err
		}
		return &Value{&Value_Array{string(ret)}}, nil
	}
	return nil, fmt.Errorf("unknown type %T", v)
}

// LiteralMap is a type for literal in-line param substitutions, or when you
// know statically that the params correspond to correct Value types.
type LiteralMap map[string]interface{}

// Convert converts this to a parameter map that can be used with
// Template.Render.
func (m LiteralMap) Convert() (map[string]*Value, error) {
	ret := make(map[string]*Value, len(m))
	for k, v := range m {
		v, err := NewValue(v)
		if err != nil {
			return nil, fmt.Errorf("key %q: %s", k, err)
		}
		ret[k] = v
	}
	return ret, nil
}

// RenderL renders this template with a LiteralMap, calling its Convert method
// and passing the result to Render.
func (t *File_Template) RenderL(m LiteralMap) (string, error) {
	pm, err := m.Convert()
	if err != nil {
		return "", err
	}
	return t.Render(pm)
}

// Render turns the Template into a JSON document, filled with the given
// parameters. It does not validate that the output is valid JSON, but if you
// called Normalize on this Template already, then it WILL be valid JSON.
func (t *File_Template) Render(params map[string]*Value) (string, error) {
	sSet := stringset.New(len(params))
	replacementSlice := make([]string, 0, len(t.Param)*2)
	for k, param := range t.Param {
		replacementSlice = append(replacementSlice, k)
		if newVal, ok := params[k]; ok {
			if err := param.Accepts(newVal); err != nil {
				return "", fmt.Errorf("param %q: %s", k, err)
			}
			sSet.Add(k)
			replacementSlice = append(replacementSlice, newVal.JSONRender())
		} else if param.Default != nil {
			replacementSlice = append(replacementSlice, param.Default.JSONRender())
		} else {
			return "", fmt.Errorf("param %q: missing", k)
		}
	}
	if len(params) != sSet.Len() {
		unknown := make([]string, 0, len(params))
		for k := range params {
			if !sSet.Has(k) {
				unknown = append(unknown, k)
			}
		}
		sort.Strings(unknown)
		return "", fmt.Errorf("unknown parameters: %q", unknown)
	}

	r := strings.NewReplacer(replacementSlice...)

	return r.Replace(t.Body), nil
}

func (v *Value) schemaType() isSchema_Schema {
	switch v.Value.(type) {
	case *Value_Str:
		return (*Schema_Str)(nil)
	case *Value_Bytes:
		return (*Schema_Bytes)(nil)
	case *Value_Int:
		return (*Schema_Int)(nil)
	case *Value_Uint:
		return (*Schema_Uint)(nil)
	case *Value_Float:
		return (*Schema_Float)(nil)
	case *Value_Bool:
		return (*Schema_Bool)(nil)
	case *Value_Object:
		return (*Schema_Object)(nil)
	case *Value_Array:
		return (*Schema_Array)(nil)
	}
	panic(fmt.Errorf("unknown type %T", v.Value))
}

func schemaTypeStr(s isSchema_Schema) string {
	switch s.(type) {
	case *Schema_Str:
		return "str"
	case *Schema_Bytes:
		return "bytes"
	case *Schema_Int:
		return "int"
	case *Schema_Uint:
		return "uint"
	case *Schema_Float:
		return "float"
	case *Schema_Bool:
		return "bool"
	case *Schema_Enum:
		return "enum"
	case *Schema_Object:
		return "object"
	case *Schema_Array:
		return "array"
	}
	panic(fmt.Errorf("unknown type %T", s))
}

// JSONRender returns the to-be-injected string rendering of v.
func (v *Value) JSONRender() string {
	return v.Value.(interface {
		JSONRender() string
	}).JSONRender()
}

// JSONRender returns a rendering of this string as JSON, e.g. go value "foo"
// renders as `"foo"`.
func (v *Value_Str) JSONRender() string {
	ret, err := json.Marshal(v.Str)
	if err != nil {
		panic(err)
	}
	return string(ret)
}

// JSONRender returns a rendering of these bytes as JSON, e.g. go value
// []byte("foo") renders as `"Zm9v"`.
func (v *Value_Bytes) JSONRender() string {
	ret, err := json.Marshal(v.Bytes)
	if err != nil {
		panic(err)
	}
	return string(ret)
}

// JSONRender returns a rendering of this int as JSON, e.g. go value 100
// renders as `100`. If the absolute value is > 2**53, this will render it as
// a string.
//
// Integers render as strings to avoid encoding issues in JSON, which only
// supports double-precision floating point numbers.
func (v *Value_Int) JSONRender() string {
	num := strconv.FormatInt(v.Int, 10)
	abs := v.Int
	if abs < 0 {
		abs = -abs
	}
	if abs < (1 << 53) {
		return num
	}
	return fmt.Sprintf(`"%s"`, num)
}

// JSONRender returns a rendering of this uint as JSON, e.g. go value 100
// renders as `"100"`.
//
// Unsigns render as strings to avoid encoding issues in JSON, which only
// supports double-precision floating point numbers.
func (v *Value_Uint) JSONRender() string {
	num := strconv.FormatUint(v.Uint, 10)
	if v.Uint < (1 << 53) {
		return num
	}
	return fmt.Sprintf(`"%s"`, num)
}

// JSONRender returns a rendering of this float as JSON, e.g. go value 1.23
// renders as `1.23`.
func (v *Value_Float) JSONRender() string {
	ret, err := json.Marshal(v.Float)
	if err != nil {
		panic(err)
	}
	return string(ret)
}

// JSONRender returns a rendering of this bool as JSON, e.g. go value true
// renders as `true`.
func (v *Value_Bool) JSONRender() string {
	if v.Bool {
		return "true"
	}
	return "false"
}

// JSONRender returns a rendering of this JSON object as JSON. This is a direct
// return of the JSON encoded string; no validation is done. To check that the
// contained string is valid, use the Valid() method.
func (v *Value_Object) JSONRender() string {
	return v.Object
}

// JSONRender returns a rendering of this JSON array as JSON. This is a direct
// return of the JSON encoded string; no validation is done. To check that the
// contained string is valid, use the Valid() method.
func (v *Value_Array) JSONRender() string {
	return v.Array
}

// JSONRender returns a rendering of null. This always returns `null`.
func (v *Value_Null) JSONRender() string {
	return "null"
}
