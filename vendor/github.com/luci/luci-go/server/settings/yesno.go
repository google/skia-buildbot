// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package settings

import (
	"errors"
)

// YesOrNo is a bool that serializes to 'yes' or 'no'.
//
// Useful in UIFields that define boolean values.
type YesOrNo bool

// String returns "yes" or "no".
func (yn YesOrNo) String() string {
	if yn {
		return "yes"
	}
	return "no"
}

// Set changes the value of YesOrNo.
func (yn *YesOrNo) Set(v string) error {
	switch v {
	case "yes":
		*yn = true
	case "no":
		*yn = false
	default:
		return errors.New("expecting 'yes' or 'no'")
	}
	return nil
}

// YesOrNoField modifies the field so that it corresponds to YesOrNo value.
//
// It sets 'Type', 'ChoiceVariants' and 'Validator' properties.
func YesOrNoField(f UIField) UIField {
	f.Type = UIFieldChoice
	f.ChoiceVariants = []string{"yes", "no"}
	f.Validator = func(v string) error {
		var x YesOrNo
		return x.Set(v)
	}
	return f
}
