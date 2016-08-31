// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package dm

import (
	"fmt"

	"github.com/luci/luci-go/common/errors"
)

// Normalize returns an error iff this GraphQuery is not valid.
func (g *GraphQuery) Normalize() error {
	if err := g.AttemptList.Normalize(); err != nil {
		return err
	}
	if len(g.AttemptRange) > 0 {
		lme := errors.NewLazyMultiError(len(g.AttemptRange))
		for i, rng := range g.AttemptRange {
			lme.Assign(i, rng.Normalize())
		}
		if err := lme.Get(); err != nil {
			return err
		}
	}
	if len(g.Search) > 0 {
		lme := errors.NewLazyMultiError(len(g.Search))
		for i, s := range g.Search {
			lme.Assign(i, s.Normalize())
		}
		if err := lme.Get(); err != nil {
			return err
		}
	}
	return nil
}

// Normalize returns nil iff this AttemptRange is in a bad state.
func (al *GraphQuery_AttemptRange) Normalize() error {
	if al.Quest == "" {
		return fmt.Errorf("must specify quest")
	}
	if al.Low == 0 {
		return fmt.Errorf("must specify low")
	}
	if al.High <= al.Low {
		return fmt.Errorf("high must be > low")
	}
	return nil
}

// Normalize returns nil iff this Search is in a bad state.
func (s *GraphQuery_Search) Normalize() error {
	// for now, start and end MUST be timestamp values.
	switch s.Start.Value.(type) {
	case *PropertyValue_Time:
	default:
		return fmt.Errorf("invalid Start type: %T", s.Start.Value)
	}
	switch s.End.Value.(type) {
	case *PropertyValue_Time:
	default:
		return fmt.Errorf("invalid End type: %T", s.End.Value)
	}
	return nil
}
