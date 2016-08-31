// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package pubsub

import (
	"flag"
)

// Subscription is a Pub/Sub subscription name.
type Subscription string

var _ flag.Value = (*Subscription)(nil)

// NewSubscription generates a new Subscritpion for a given project and
// subscription name.
func NewSubscription(project, name string) Subscription {
	return Subscription(newResource(project, "subscriptions", name))
}

func (s *Subscription) String() string {
	return string(*s)
}

// Set implements flag.Value.
func (s *Subscription) Set(value string) error {
	v := Subscription(value)
	if err := v.Validate(); err != nil {
		return err
	}
	*s = v
	return nil
}

// Validate returns an error if the subscription name is invalid.
func (s Subscription) Validate() error {
	return validateResource(string(s), "subscriptions")
}

// Split returns the Subscription's project component. If no project is
// defined (malformed), an empty string will be returned.
func (s Subscription) Split() (p, n string) {
	p, n, _ = s.SplitErr()
	return
}

// SplitErr returns the Subscription's project and name components.
func (s Subscription) SplitErr() (p, n string, err error) {
	p, n, err = resourceProjectName(string(s))
	return
}
