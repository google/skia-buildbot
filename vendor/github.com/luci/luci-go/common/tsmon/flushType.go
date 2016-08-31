// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package tsmon

import (
	"flag"

	"github.com/luci/luci-go/common/flag/flagenum"
)

// FlushType is a flush type enumeration.
type FlushType string

var _ flag.Value = (*FlushType)(nil)

const (
	// FlushManual requires the user to flush manually.
	FlushManual = FlushType("manual")
	// FlushAuto automatically flushes at a periodic flush interval.
	FlushAuto = FlushType("auto")
)

var flushTypeEnum = flagenum.Enum{
	"manual": FlushManual,
	"auto":   FlushAuto,
}

func (ft *FlushType) String() string {
	return flushTypeEnum.FlagString(ft)
}

// Set implements flag.Value.
func (ft *FlushType) Set(v string) error {
	return flushTypeEnum.FlagSet(ft, v)
}
