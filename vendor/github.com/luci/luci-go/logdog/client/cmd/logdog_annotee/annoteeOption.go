// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package main

import (
	"flag"

	"github.com/luci/luci-go/common/flag/flagenum"
	"github.com/luci/luci-go/logdog/client/annotee/executor"
)

type annotationMode executor.AnnotationMode

var annotationFlagEnum = flagenum.Enum{
	"none":  annotationMode(executor.NoAnnotations),
	"tee":   annotationMode(executor.TeeAnnotations),
	"strip": annotationMode(executor.StripAnnotations),
}

var _ flag.Value = (*annotationMode)(nil)

func (val *annotationMode) Set(v string) error {
	return annotationFlagEnum.FlagSet(val, v)
}

func (val *annotationMode) String() string {
	return annotationFlagEnum.FlagString(val)
}
