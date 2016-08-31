// Copyright 2016 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

package types

import (
	"fmt"

	pb "github.com/luci/luci-go/common/tsmon/ts_mon_proto"
)

// MetricDataUnits are enums for the units of metrics data.
type MetricDataUnits int32

// Units of metrics data.
const (
	// NotSpecified is a magic value to skip Units property
	// in a MetricsData proto message.
	// Note that this should be defined with 0 so that all the existing
	// codes without units specified would be given NotSpecified
	// for the Units property automatically.
	NotSpecified MetricDataUnits = iota
	Unknown
	Seconds
	Milliseconds
	Microseconds
	Nanoseconds
	Bits
	Bytes

	Kilobytes // 1000 bytes (not 1024).
	Megabytes // 1e6 (1,000,000) bytes.
	Gigabytes // 1e9 (1,000,000,000) bytes.
	Kibibytes // 1024 bytes.
	Mebibytes // 1024^2 (1,048,576) bytes.
	Gibibytes // 1024^3 (1,073,741,824) bytes.

	// * Extended Units
	AmpUnit
	MilliampUnit
	DegreeCelsiusUnit
)

// IsSpecified returns true if a unit annotation has been specified
// for a given metric.
func (units MetricDataUnits) IsSpecified() bool {
	return units != NotSpecified
}

// AsProto returns the protobuf representation of a given MetricDataUnits
// object.
func (units MetricDataUnits) AsProto() *pb.MetricsData_Units {
	switch units {
	case Unknown:
		return pb.MetricsData_UNKNOWN_UNITS.Enum()
	case Seconds:
		return pb.MetricsData_SECONDS.Enum()
	case Milliseconds:
		return pb.MetricsData_MILLISECONDS.Enum()
	case Microseconds:
		return pb.MetricsData_MICROSECONDS.Enum()
	case Nanoseconds:
		return pb.MetricsData_NANOSECONDS.Enum()
	case Bits:
		return pb.MetricsData_BITS.Enum()
	case Bytes:
		return pb.MetricsData_BYTES.Enum()
	case Kilobytes:
		return pb.MetricsData_KILOBYTES.Enum()
	case Megabytes:
		return pb.MetricsData_MEGABYTES.Enum()
	case Gigabytes:
		return pb.MetricsData_GIGABYTES.Enum()
	case Kibibytes:
		return pb.MetricsData_KIBIBYTES.Enum()
	case Mebibytes:
		return pb.MetricsData_MEBIBYTES.Enum()
	case Gibibytes:
		return pb.MetricsData_GIBIBYTES.Enum()
	case AmpUnit:
		return pb.MetricsData_AMPS.Enum()
	case MilliampUnit:
		return pb.MetricsData_MILLIAMPS.Enum()
	case DegreeCelsiusUnit:
		return pb.MetricsData_DEGREES_CELSIUS.Enum()
	}
	panic(fmt.Sprintf("unknown MetricDataUnits %d", units))
}
