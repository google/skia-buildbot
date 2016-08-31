// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package iam

import (
	"bytes"
	"encoding/json"
	"sort"
)

// Policy is an IAM policy object.
//
// See https://cloud.google.com/iam/reference/rest/v1/Policy.
type Policy struct {
	Bindings PolicyBindings
	Etag     string

	// All other JSON fields we are not interested in but must to preserve.
	//
	// They are assumed to be immutable. Clone and Equals below treat them as
	// scalar values, not as pointers to []byte.
	UnrecognizedFields map[string]*json.RawMessage
}

// MarshalJSON is part of json.Marshaler interface.
func (p Policy) MarshalJSON() ([]byte, error) {
	// We have to use *json.RawMessage instead of json.RawMessage because
	// otherwise json reflection magic gets confused and serializes
	// json.RawMessage as []byte. See http://play.golang.org/p/iid7Xthp-_.
	fields := make(map[string]*json.RawMessage, len(p.UnrecognizedFields)+2)
	for k, v := range p.UnrecognizedFields {
		fields[k] = v
	}

	blob, err := json.Marshal(p.Bindings)
	if err != nil {
		return nil, err
	}
	bindings := json.RawMessage(blob)
	fields["bindings"] = &bindings

	blob, err = json.Marshal(p.Etag)
	if err != nil {
		return nil, err
	}
	etag := json.RawMessage(blob)
	fields["etag"] = &etag

	return json.Marshal(fields)
}

// UnmarshalJSON is part of json.Unmarshaler interface.
func (p *Policy) UnmarshalJSON(data []byte) error {
	var fields map[string]*json.RawMessage
	if err := json.Unmarshal(data, &fields); err != nil {
		return err
	}

	newOne := Policy{
		Bindings:           make(PolicyBindings),
		UnrecognizedFields: fields,
	}

	if raw, ok := fields["bindings"]; ok && raw != nil {
		delete(fields, "bindings")
		if err := json.Unmarshal([]byte(*raw), &newOne.Bindings); err != nil {
			return err
		}
	}

	if raw, ok := fields["etag"]; ok && raw != nil {
		delete(fields, "etag")
		if err := json.Unmarshal([]byte(*raw), &newOne.Etag); err != nil {
			return err
		}
	}

	*p = newOne
	return nil
}

// Clone makes a deep copy of this object.
func (p Policy) Clone() Policy {
	c := p
	if p.Bindings != nil {
		c.Bindings = p.Bindings.Clone()
	}
	if p.UnrecognizedFields != nil {
		c.UnrecognizedFields = make(map[string]*json.RawMessage, len(p.UnrecognizedFields))
		for k, v := range p.UnrecognizedFields {
			c.UnrecognizedFields[k] = v
		}
	}
	return c
}

// Equals returns true if this object is equal to another one.
func (p Policy) Equals(another Policy) bool {
	if p.Etag != another.Etag {
		return false
	}

	if !p.Bindings.Equals(another.Bindings) {
		return false
	}

	if len(p.UnrecognizedFields) != len(another.UnrecognizedFields) {
		return false
	}
	for k, left := range p.UnrecognizedFields {
		right, hasIt := another.UnrecognizedFields[k]
		if !hasIt || !isEqualRawMsg(left, right) {
			return false
		}
	}

	return true
}

func isEqualRawMsg(left, right *json.RawMessage) bool {
	switch {
	case left == right:
		return true
	case left == nil && right != nil:
		return false
	case left != nil && right == nil:
		return false
	default:
		return bytes.Equal([]byte(*left), []byte(*right))
	}
}

// GrantRole grants a role to the given set of principals.
func (p *Policy) GrantRole(role string, principals ...string) {
	if len(principals) == 0 {
		return
	}
	if p.Bindings == nil {
		p.Bindings = make(PolicyBindings, 1)
	}
	members := p.Bindings[role]
	if members == nil {
		members = make(membersSet, len(principals))
		p.Bindings[role] = members
	}
	for _, principal := range principals {
		members[principal] = struct{}{}
	}
}

// RevokeRole removes a role from the given set of principals.
func (p *Policy) RevokeRole(role string, principals ...string) {
	members := p.Bindings[role]
	for _, principal := range principals {
		delete(members, principal)
	}
}

// membersSet is a set of members that have been granted some role.
type membersSet map[string]struct{}

// PolicyBindings is the IAM policy map {role -> set of members}.
//
// Implements json.Marshaler and json.Unmarshaler.
type PolicyBindings map[string]membersSet

type roleBinding struct {
	Role    string   `json:"role"`
	Members []string `json:"members,omitempty"`
}

// MarshalJSON is part of json.Marshaler interface.
func (b PolicyBindings) MarshalJSON() ([]byte, error) {
	roles := make([]string, 0, len(b))
	for role, members := range b {
		if len(members) != 0 {
			roles = append(roles, role)
		}
	}
	sort.Strings(roles)

	bindings := make([]roleBinding, 0, len(roles))
	for _, role := range roles {
		members := make([]string, 0, len(b[role]))
		for k := range b[role] {
			members = append(members, k)
		}
		sort.Strings(members)
		bindings = append(bindings, roleBinding{
			Role:    role,
			Members: members,
		})
	}

	return json.Marshal(bindings)
}

// UnmarshalJSON is part of json.Unmarshaler interface.
func (b *PolicyBindings) UnmarshalJSON(data []byte) error {
	bindings := []roleBinding{}
	if err := json.Unmarshal(data, &bindings); err != nil {
		return err
	}
	newOne := make(PolicyBindings, len(bindings))
	for _, item := range bindings {
		members := make(membersSet, len(item.Members))
		for _, member := range item.Members {
			members[member] = struct{}{}
		}
		newOne[item.Role] = members
	}
	*b = newOne
	return nil
}

// Clone makes a deep copy of this object.
func (b PolicyBindings) Clone() PolicyBindings {
	clone := make(PolicyBindings, len(b))
	for role, members := range b {
		membersCopy := make(membersSet, len(members))
		for k, v := range members {
			membersCopy[k] = v
		}
		clone[role] = membersCopy
	}
	return clone
}

// Equals returns true if this object is equal to another one.
func (b PolicyBindings) Equals(another PolicyBindings) bool {
	if len(b) != len(another) {
		return false
	}
	for role, right := range another {
		left := b[role]
		if len(left) != len(right) {
			return false
		}
		for item := range left {
			if _, hasIt := right[item]; !hasIt {
				return false
			}
		}
	}
	return true
}
