// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package secrets

import (
	"errors"
)

var (
	// ErrNoSuchSecret is returned by Store.GetSecret if it can't find a secret.
	ErrNoSuchSecret = errors.New("secret not found")
)

// Key names a secret.
type Key string

// NamedBlob is byte buffer with an ID string that identifies this particular
// version of the secret.
type NamedBlob struct {
	ID   string // short human readable URL safe string
	Blob []byte // actual secret blob, size depends on Store implementation
}

// Clone makes a deep copy of the NamedBlob.
func (b NamedBlob) Clone() NamedBlob {
	return NamedBlob{
		ID:   b.ID,
		Blob: append([]byte(nil), b.Blob...),
	}
}

// Secret represents a current value of a secret as well as a set of few
// previous values. Previous values are important when key is being rotated:
// there may be valid outstanding derivatives of previous values of the secret.
//
// Each value (current and previous) have an identifier that can be put into
// derived messages to name specific version of the value.
type Secret struct {
	Current  NamedBlob   // current value of the secret, always set
	Previous []NamedBlob // optional list of previous values, most recent first
}

// Blobs returns current blob and all previous blobs as one array.
func (s Secret) Blobs() []NamedBlob {
	out := make([]NamedBlob, 0, 1+len(s.Previous))
	out = append(out, s.Current)
	out = append(out, s.Previous...)
	return out
}

// Clone makes a deep copy of the Secret.
func (s Secret) Clone() Secret {
	out := Secret{Current: s.Current.Clone()}
	if s.Previous != nil {
		out.Previous = make([]NamedBlob, len(s.Previous))
		for i := range out.Previous {
			out.Previous[i] = s.Previous[i].Clone()
		}
	}
	return out
}

// Store knows how to retrieve (or autogenerate) a secret given its key.
type Store interface {
	// GetSecret returns a secret given its key. Store may choose to autogenerate
	// a secret if there's no existing one, or it may choose to treat it as a
	// error and return ErrNoSuchSecret. Returned secret is always a mutable copy
	// of an actual secret in the Store's gut (just a precaution against
	// unintended modifications of arrays that back all byte blobs).
	GetSecret(Key) (Secret, error)
}

// StaticStore is Store with predefined secrets.
type StaticStore map[Key]Secret

// GetSecret returns a copy of a secret given its key or ErrNoSuchSecret if no
// such secret.
func (s StaticStore) GetSecret(k Key) (Secret, error) {
	if secret, ok := s[k]; ok {
		return secret.Clone(), nil
	}
	return Secret{}, ErrNoSuchSecret
}
