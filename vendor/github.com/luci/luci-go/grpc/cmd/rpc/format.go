// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"flag"

	"github.com/luci/luci-go/common/flag/flagenum"
	"github.com/luci/luci-go/grpc/prpc"
)

type formatFlag prpc.Format

const formatFlagPB formatFlag = -1

// formatFlag is a CLI-settable pRPC format flag.
var formatFlagMap = flagenum.Enum{
	"flag":   formatFlagPB,
	"binary": formatFlag(prpc.FormatBinary),
	"json":   formatFlag(prpc.FormatJSONPB),
	"text":   formatFlag(prpc.FormatText),
}

var _ flag.Value = (*formatFlag)(nil)

func (f formatFlag) Format() prpc.Format {
	return prpc.Format(f)
}

func (f *formatFlag) String() string     { return formatFlagMap.FlagString(f) }
func (f *formatFlag) Set(v string) error { return formatFlagMap.FlagSet(f, v) }
