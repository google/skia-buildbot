// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package templateproto

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"reflect"
	"regexp"
	"strings"

	"github.com/luci/luci-go/common/data/stringset"
	"github.com/luci/luci-go/common/errors"
)

// Normalize will normalize all of the Templates in this message, returning an
// error if any are invalid.
func (f *File) Normalize() error {
	me := errors.MultiError(nil)
	for tname, t := range f.Template {
		if err := t.Normalize(); err != nil {
			me = append(me, fmt.Errorf("for template %q: %s", tname, err))
		}
	}
	if len(me) > 0 {
		return me
	}
	return nil
}

// ParamRegex is the regular expression that all parameter names must match.
var ParamRegex = regexp.MustCompile(`^\${[^}]+}$`)

// Normalize will normalize the Template message, returning an error if it is
// invalid.
func (t *File_Template) Normalize() error {
	if t.Body == "" {
		return errors.New("body is empty")
	}

	defaultParams := make(map[string]*Value, len(t.Param))
	for k, param := range t.Param {
		if k == "" {
			return fmt.Errorf("param %q: invalid name", k)
		}
		if !ParamRegex.MatchString(k) {
			return fmt.Errorf("param %q: malformed name", k)
		}
		if !strings.Contains(t.Body, k) {
			return fmt.Errorf("param %q: not present in body", k)
		}
		if err := param.Normalize(); err != nil {
			return fmt.Errorf("param %q: %s", k, err)
		}
		if param.Default != nil {
			defaultParams[k] = param.Default
		} else {
			defaultParams[k] = param.Schema.Zero()
		}
	}

	maybeJSON, err := t.Render(defaultParams)
	if err != nil {
		return fmt.Errorf("rendering: %s", err)
	}

	err = json.Unmarshal([]byte(maybeJSON), &map[string]interface{}{})
	if err != nil {
		return fmt.Errorf("parsing rendered body: %s", err)
	}
	return nil
}

// Normalize will normalize the Parameter, returning an error if it is invalid.
func (p *File_Template_Parameter) Normalize() error {
	if p == nil {
		return errors.New("is nil")
	}
	if err := p.Schema.Normalize(); err != nil {
		return fmt.Errorf("schema: %s", err)
	}
	if p.Default != nil {
		if err := p.Default.Normalize(); err != nil {
			return fmt.Errorf("default value: %s", err)
		}
		if err := p.Accepts(p.Default); err != nil {
			return fmt.Errorf("default value: %s", err)
		}
	}
	return nil
}

// Accepts returns nil if this Parameter can accept the Value.
func (p *File_Template_Parameter) Accepts(v *Value) error {
	if v.IsNull() {
		if !p.Nullable {
			return errors.New("not nullable")
		}
	} else if err := p.Schema.Accepts(v); err != nil {
		return err
	} else if err := v.Check(p.Schema); err != nil {
		return err
	}
	return nil
}

// Normalize will normalize the Schema, returning an error if it is invalid.
func (s *Schema) Normalize() error {
	if s == nil {
		return errors.New("is nil")
	}
	if s.Schema == nil {
		return errors.New("has no type")
	}
	if enum := s.GetEnum(); enum != nil {
		return enum.Normalize()
	}
	return nil
}

// Normalize will normalize the Schema_Set, returning an error if it is
// invalid.
func (s *Schema_Set) Normalize() error {
	if len(s.Entry) == 0 {
		return errors.New("set requires entries")
	}
	set := stringset.New(len(s.Entry))
	for _, entry := range s.Entry {
		if entry.Token == "" {
			return errors.New("blank token")
		}
		if !set.Add(entry.Token) {
			return fmt.Errorf("duplicate token %q", entry.Token)
		}
	}
	return nil
}

// Has returns true iff the given token is a valid value for this enumeration.
func (s *Schema_Set) Has(token string) bool {
	for _, tok := range s.Entry {
		if tok.Token == token {
			return true
		}
	}
	return false
}

// IsNull returns true if this Value is the null value.
func (v *Value) IsNull() bool {
	_, ret := v.Value.(*Value_Null)
	return ret
}

// Check ensures that this value conforms to the given schema.
func (v *Value) Check(s *Schema) error {
	check, needsCheck := v.Value.(interface {
		Check(*Schema) error
	})
	if !needsCheck {
		return nil
	}
	return check.Check(s)
}

// Normalize returns a non-nil error if the Value is invalid for its nominal type.
func (v *Value) Normalize() error {
	norm, needsNormalization := v.Value.(interface {
		Normalize() error
	})
	if !needsNormalization {
		return nil
	}
	return norm.Normalize()
}

// Check returns nil iff this Value meets the max length criteria.
func (v *Value_Bytes) Check(schema *Schema) error {
	s := schema.GetBytes()
	if s.MaxLength > 0 && uint32(len(v.Bytes)) > s.MaxLength {
		return fmt.Errorf("value is too large: %d > %d", len(v.Bytes), s.MaxLength)
	}
	return nil
}

// Check returns nil iff this Value meets the max length criteria, and/or
// can be used to fill an enumeration value from the provided schema.
func (v *Value_Str) Check(schema *Schema) error {
	switch s := schema.Schema.(type) {
	case *Schema_Str:
		maxLen := s.Str.MaxLength
		if maxLen > 0 && uint32(len(v.Str)) > maxLen {
			return fmt.Errorf("value is too large: %d > %d", len(v.Str), maxLen)
		}

	case *Schema_Enum:
		if !s.Enum.Has(v.Str) {
			return fmt.Errorf("value does not match enum: %q", v.Str)
		}

	default:
		panic(fmt.Errorf("for Value_Str: unknown schema %T", s))
	}

	return nil
}

// Check returns nil iff this Value correctly parses as a JSON object.
func (v *Value_Object) Check(schema *Schema) error {
	s := schema.GetObject()
	if s.MaxLength > 0 && uint32(len(v.Object)) > s.MaxLength {
		return fmt.Errorf("value is too large: %d > %d", len(v.Object), s.MaxLength)
	}
	return nil
}

// Normalize returns nil iff this Value correctly parses as a JSON object.
func (v *Value_Object) Normalize() error {
	newObj, err := NormalizeJSON(v.Object, true)
	if err != nil {
		return err
	}
	v.Object = newObj
	return nil
}

// Check returns nil iff this Value correctly parses as a JSON array.
func (v *Value_Array) Check(schema *Schema) error {
	s := schema.GetArray()
	if s.MaxLength > 0 && uint32(len(v.Array)) > s.MaxLength {
		return fmt.Errorf("value is too large: %d > %d", len(v.Array), s.MaxLength)
	}
	return nil
}

// Normalize returns nil iff this Value correctly parses as a JSON array.
func (v *Value_Array) Normalize() error {
	newAry, err := NormalizeJSON(v.Array, false)
	if err != nil {
		return err
	}
	v.Array = newAry
	return nil
}

var (
	typeOfSchemaStr  = reflect.TypeOf((*Schema_Str)(nil))
	typeOfSchemaEnum = reflect.TypeOf((*Schema_Enum)(nil))
)

// Accepts returns nil if this Schema can accept the Value.
func (s *Schema) Accepts(v *Value) error {
	typ := reflect.TypeOf(s.Schema)
	if typ == typeOfSchemaEnum {
		typ = typeOfSchemaStr
	}
	if typ != reflect.TypeOf(v.schemaType()) {
		return fmt.Errorf("type is %q, expected %q", schemaTypeStr(v.schemaType()), schemaTypeStr(s.Schema))
	}
	return nil
}

// Zero produces a Value from this schema which is a valid 'zero' value (in the
// go sense)
func (s *Schema) Zero() *Value {
	switch sub := s.Schema.(type) {
	case *Schema_Int:
		return MustNewValue(0)
	case *Schema_Uint:
		return MustNewValue(uint(0))
	case *Schema_Float:
		return MustNewValue(0.0)
	case *Schema_Bool:
		return MustNewValue(false)
	case *Schema_Str:
		return MustNewValue("")
	case *Schema_Bytes:
		return MustNewValue([]byte{})
	case *Schema_Enum:
		return MustNewValue(sub.Enum.Entry[0].Token)
	case *Schema_Object:
		return &Value{&Value_Object{"{}"}}
	case *Schema_Array:
		return &Value{&Value_Array{"[]"}}
	}
	panic(fmt.Errorf("unknown schema type: %v", s))
}

// NormalizeJSON is used to take some free-form JSON and validates that:
//   * it only contains a valid JSON object (e.g. `{...stuff...}`); OR
//   * it only contains a valid JSON array (e.g. `[...stuff...]`)
//
// If obj is true, this looks for an object, if it's false, it looks for an
// array.
//
// This will also remove all extra whitespace and sort all objects by key.
func NormalizeJSON(data string, obj bool) (string, error) {
	buf := bytes.NewBufferString(data)
	dec := json.NewDecoder(buf)
	dec.UseNumber()
	var decoded interface{}
	if obj {
		decoded = &map[string]interface{}{}
	} else {
		decoded = &[]interface{}{}
	}
	err := dec.Decode(decoded)
	if err != nil {
		return "", err
	}
	bufdat, err := ioutil.ReadAll(dec.Buffered())
	if err != nil {
		panic(err)
	}
	rest := strings.TrimSpace(string(bufdat) + buf.String())
	if rest != "" {
		return "", fmt.Errorf("got extra junk: %q", rest)
	}

	buf.Reset()
	err = json.NewEncoder(buf).Encode(decoded)

	// the TrimSpace chops off an extraneous newline that the json lib adds on.
	return string(bytes.TrimSpace(buf.Bytes())), err
}

// Normalize will normalize this Specifier
func (s *Specifier) Normalize() error {
	if s.TemplateName == "" {
		return errors.New("empty template_name")
	}
	for k, v := range s.Params {
		if k == "" {
			return errors.New("empty param key")
		}
		if err := v.Normalize(); err != nil {
			return fmt.Errorf("param %q: %s", k, err)
		}
	}
	return nil
}
