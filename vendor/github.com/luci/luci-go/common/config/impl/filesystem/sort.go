// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package filesystem

import (
	"github.com/luci/luci-go/common/config"
)

type configList []config.Config

func (cl configList) Len() int      { return len(cl) }
func (cl configList) Swap(i, j int) { cl[i], cl[j] = cl[j], cl[i] }
func (cl configList) Less(i, j int) bool {
	if cl[i].ConfigSet < cl[j].ConfigSet {
		return true
	} else if cl[i].ConfigSet > cl[j].ConfigSet {
		return false
	}
	return cl[i].Path < cl[j].Path
}

type projList []config.Project

func (pl projList) Len() int      { return len(pl) }
func (pl projList) Swap(i, j int) { pl[i], pl[j] = pl[j], pl[i] }
func (pl projList) Less(i, j int) bool {
	return pl[i].ID < pl[j].ID
}
