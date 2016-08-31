// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package target

import (
	"flag"

	"github.com/luci/luci-go/common/flag/flagenum"
)

// Type is a target type enumeration.
type Type string

var _ flag.Value = (*Type)(nil)

const (
	// DeviceType is a device target type.
	DeviceType = Type("device")
	// TaskType represents a task target type.
	TaskType = Type("task")
)

var targetTypeEnum = flagenum.Enum{
	"device": DeviceType,
	"task":   TaskType,
}

func (tt *Type) String() string {
	return targetTypeEnum.FlagString(tt)
}

// Set implements flag.Value.
func (tt *Type) Set(v string) error {
	return targetTypeEnum.FlagSet(tt, v)
}
