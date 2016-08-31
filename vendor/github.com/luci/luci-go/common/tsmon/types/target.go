// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package types

import (
	pb "github.com/luci/luci-go/common/tsmon/ts_mon_proto"
)

// A Target knows how to put information about itself in a MetricsData message.
type Target interface {
	PopulateProto(d *pb.MetricsData)
	IsPopulatedIn(d *pb.MetricsData) bool
	Hash() uint64
	Clone() Target
}
