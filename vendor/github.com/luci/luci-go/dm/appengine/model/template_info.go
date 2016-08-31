// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package model

import (
	"sort"
	"strings"

	dm "github.com/luci/luci-go/dm/api/service/v1"
	"github.com/xtgo/set"
)

// TemplateInfo is an ordered list of dm.Quest_TemplateSpec's
type TemplateInfo []dm.Quest_TemplateSpec

func (ti TemplateInfo) Len() int      { return len(ti) }
func (ti TemplateInfo) Swap(i, j int) { ti[i], ti[j] = ti[j], ti[i] }
func (ti TemplateInfo) Less(i, j int) bool {
	a, b := ti[i], ti[j]
	if v := strings.Compare(a.Project, b.Project); v != 0 {
		return v < 0
	}
	if v := strings.Compare(a.Ref, b.Ref); v != 0 {
		return v < 0
	}
	if v := strings.Compare(a.Version, b.Version); v != 0 {
		return v < 0
	}
	return strings.Compare(a.Name, b.Name) < 0
}

// EqualsData returns true iff this TemplateInfo has the same content as the
// proto-style TemplateInfo. This assumes that `other` is sorted.
func (ti TemplateInfo) EqualsData(other []*dm.Quest_TemplateSpec) bool {
	if len(other) != len(ti) {
		return false
	}
	for i, me := range ti {
		if !me.Equals(other[i]) {
			return false
		}
	}
	return true
}

// Add adds ts to the TemplateInfo uniq'ly.
func (ti *TemplateInfo) Add(ts ...dm.Quest_TemplateSpec) {
	*ti = append(*ti, ts...)
	sort.Sort(*ti)
	*ti = (*ti)[:set.Uniq(*ti)]
}
