// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package identity

import (
	"fmt"
	"regexp"
	"strings"
)

// Kind is enumeration of known identity kinds. See Identity.
type Kind string

const (
	// Anonymous kind means no identity information is provided. Identity value
	// is always 'anonymous'.
	Anonymous Kind = "anonymous"

	// Bot is used for bots authenticated via IP whitelist. Identity value is
	// bot's IP address (IPv4 or IPv6).
	Bot Kind = "bot"

	// Service is used for GAE apps using X-Appengine-Inbound-Appid header for
	// authentication. Identity value is GAE app id.
	Service Kind = "service"

	// User is used for regular users. Identity value is email address.
	User Kind = "user"
)

// knownKinds is used in Validate. It is mapping between Kind and regexp for
// identity value. See also appengine/components/components/auth/model.py in
// luci-py.
var knownKinds = map[Kind]*regexp.Regexp{
	Anonymous: regexp.MustCompile(`^anonymous$`),
	Bot:       regexp.MustCompile(`^[0-9a-zA-Z_\-\.@]+$`),
	Service:   regexp.MustCompile(`^[0-9a-zA-Z_\-\:\.]+$`),
	User:      regexp.MustCompile(`^[0-9a-zA-Z_\-\.\+]+@[0-9a-z_\-\.]+$`),
}

// Identity represents a caller that makes requests. A string of the form
// "kind:value" where 'kind' is one of Kind constant and meaning of 'value'
// depends on a kind (see comments for Kind values).
type Identity string

// AnonymousIdentity represents anonymous user.
const AnonymousIdentity Identity = "anonymous:anonymous"

// MakeIdentity ensures 'identity' string looks like a valid identity and
// returns it as Identity value.
func MakeIdentity(identity string) (Identity, error) {
	id := Identity(identity)
	if err := id.Validate(); err != nil {
		return "", err
	}
	return id, nil
}

// Validate checks that the identity string is well-formed.
func (id Identity) Validate() error {
	chunks := strings.SplitN(string(id), ":", 2)
	if len(chunks) != 2 {
		return fmt.Errorf("auth: bad identity string %q", id)
	}
	re := knownKinds[Kind(chunks[0])]
	if re == nil {
		return fmt.Errorf("auth: bad identity kind %q", chunks[0])
	}
	if !re.MatchString(chunks[1]) {
		return fmt.Errorf("auth: bad value %q for identity kind %q", chunks[1], chunks[0])
	}
	return nil
}

// Kind returns identity kind. If identity string is invalid returns Anonymous.
func (id Identity) Kind() Kind {
	chunks := strings.SplitN(string(id), ":", 2)
	if len(chunks) != 2 {
		return Anonymous
	}
	return Kind(chunks[0])
}

// Value returns a valued encoded in the identity, e.g. for User identity kind
// it is user's email address.If identity string is invalid returns "anonymous".
func (id Identity) Value() string {
	chunks := strings.SplitN(string(id), ":", 2)
	if len(chunks) != 2 {
		return "anonymous"
	}
	return chunks[1]
}

// Email returns user's email for identity with kind User or empty string for
// all other identity kinds. If identity string is undefined returns "".
func (id Identity) Email() string {
	chunks := strings.SplitN(string(id), ":", 2)
	if len(chunks) != 2 {
		return ""
	}
	if Kind(chunks[0]) == User {
		return chunks[1]
	}
	return ""
}
