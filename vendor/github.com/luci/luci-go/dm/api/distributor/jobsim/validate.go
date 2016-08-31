// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package jobsim

import (
	"fmt"
)

var _ interface {
	Normalize() error
} = (*Phrase)(nil)

// Normalize checks Phrase for errors and converts it to its normalized form.
func (p *Phrase) Normalize() error {
	if p != nil {
		if p.Name == "" {
			return fmt.Errorf("empty ID")
		}
		for _, s := range p.Stages {
			if err := s.Normalize(); err != nil {
				return err
			}
		}
		if p.ReturnStage == nil {
			p.ReturnStage = &ReturnStage{1, nil}
		}
	}
	return nil
}

// Normalize checks Stage for errors and converts it to its normalized form.
func (s *Stage) Normalize() error {
	switch st := s.GetStageType().(type) {
	case *Stage_Failure:
		return st.Failure.Normalize()
	case *Stage_Stall:
		return st.Stall.Normalize()
	case *Stage_Deps:
		return st.Deps.Normalize()
	}
	return fmt.Errorf("empty stage")
}

// Normalize checks FailureStage for errors and converts it to its normalized
// form.
func (f *FailureStage) Normalize() error {
	if f.Chance < 0 {
		return fmt.Errorf("too small FailureStage chance")
	}
	if f.Chance > 1 {
		return fmt.Errorf("too large FailureStage chance")
	}
	return nil
}

// Normalize checks StallStage for errors and converts it to its normalized form.
func (s *StallStage) Normalize() error {
	// nothing to do
	return nil
}

// Normalize checks DepsStage for errors and converts it to its normalized form.
func (d *DepsStage) Normalize() error {
	for _, d := range d.Deps {
		if err := d.Normalize(); err != nil {
			return err
		}
	}
	return nil
}

// Normalize checks Dependency for errors and converts it to its normalized form.
func (d *Dependency) Normalize() error {
	if d.Shards == 0 {
		d.Shards = 1
	}
	if err := d.GetAttempts().Normalize(); err != nil {
		return err
	}
	return d.Phrase.Normalize()
}

// Normalize checks SparseRange for errors and converts it to its normalized form.
func (s *SparseRange) Normalize() error {
	if len(s.Items) == 0 {
		s.Items = []*RangeItem{{&RangeItem_Single{1}}}
		return nil
	}
	current := uint32(0)
	for _, itm := range s.Items {
		switch i := itm.RangeItem.(type) {
		case *RangeItem_Single:
			if i.Single <= current {
				return fmt.Errorf("malformed SparseRange")
			}
			current = i.Single
		case *RangeItem_Range:
			if err := i.Range.Normalize(); err != nil {
				return err
			}
			if i.Range.Low <= current {
				return fmt.Errorf("malformed SparseRange")
			}
			current = i.Range.High
		default:
			return fmt.Errorf("unknown SparseRange entry")
		}
	}
	return nil
}

// Normalize checks Range for errors and converts it to its normalized form.
func (r *Range) Normalize() error {
	if r.High <= r.Low {
		return fmt.Errorf("malformed Range")
	}
	return nil
}
