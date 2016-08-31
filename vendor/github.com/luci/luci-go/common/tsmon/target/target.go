// Copyright 2015 The LUCI Authors. All rights reserved.
// Use of this source code is governed under the Apache License, Version 2.0
// that can be found in the LICENSE file.

// Package target contains information about the thing that is sending metrics -
// either a NetworkDevice (a machine) or a Task (a service).
// There is a default target that is usually configured with commandline flags
// (flags.go), but a target can also be passed through the Context (context.go)
// if you need to set metric values for a different target.
package target

import (
	"errors"
	"fmt"
	"hash/fnv"

	"github.com/golang/protobuf/proto"
	"github.com/luci/luci-go/common/tsmon/types"

	pb "github.com/luci/luci-go/common/tsmon/ts_mon_proto"
)

// A Task is a process or a service running on one or more machine.
type Task pb.Task

// AsProto returns this object as a tsmon.proto.Task message.
func (t *Task) AsProto() *pb.Task { return (*pb.Task)(t) }

// PopulateProto implements Target.
func (t *Task) PopulateProto(d *pb.MetricsData) { d.Task = t.AsProto() }

// IsPopulatedIn returns true if the MetricsData message is for this target.
func (t *Task) IsPopulatedIn(d *pb.MetricsData) bool {
	return d.Task != nil &&
		d.Task.GetServiceName() == t.AsProto().GetServiceName() &&
		d.Task.GetJobName() == t.AsProto().GetJobName() &&
		d.Task.GetDataCenter() == t.AsProto().GetDataCenter() &&
		d.Task.GetHostName() == t.AsProto().GetHostName() &&
		d.Task.GetTaskNum() == t.AsProto().GetTaskNum()
}

// Hash returns a uint64 hash of this target.
func (t *Task) Hash() uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s\n%s\n%s\n%s\n%d",
		t.AsProto().GetServiceName(),
		t.AsProto().GetJobName(),
		t.AsProto().GetDataCenter(),
		t.AsProto().GetHostName(),
		t.AsProto().GetTaskNum())
	return h.Sum64()
}

// Clone returns a copy of this object.
func (t *Task) Clone() types.Target {
	return (*Task)(proto.Clone(t.AsProto()).(*pb.Task))
}

// A NetworkDevice is a machine that has a hostname.
type NetworkDevice pb.NetworkDevice

// AsProto returns this object as a tsmon.proto.NetworkDevice message.
func (t *NetworkDevice) AsProto() *pb.NetworkDevice { return (*pb.NetworkDevice)(t) }

// PopulateProto implements Target.
func (t *NetworkDevice) PopulateProto(d *pb.MetricsData) { d.NetworkDevice = t.AsProto() }

// IsPopulatedIn returns true if the MetricsData message is for this target.
func (t *NetworkDevice) IsPopulatedIn(d *pb.MetricsData) bool {
	return d.NetworkDevice != nil &&
		d.NetworkDevice.GetAlertable() == t.AsProto().GetAlertable() &&
		d.NetworkDevice.GetRealm() == t.AsProto().GetRealm() &&
		d.NetworkDevice.GetMetro() == t.AsProto().GetMetro() &&
		d.NetworkDevice.GetRole() == t.AsProto().GetRole() &&
		d.NetworkDevice.GetHostname() == t.AsProto().GetHostname() &&
		d.NetworkDevice.GetHostgroup() == t.AsProto().GetHostgroup()
}

// Hash returns a uint64 hash of this target.
func (t *NetworkDevice) Hash() uint64 {
	h := fnv.New64a()
	fmt.Fprintf(h, "%t%s\n%s\n%s\n%s\n%s",
		t.AsProto().GetAlertable(),
		t.AsProto().GetRealm(),
		t.AsProto().GetMetro(),
		t.AsProto().GetRole(),
		t.AsProto().GetHostname(),
		t.AsProto().GetHostgroup())
	return h.Sum64()
}

// Clone returns a copy of this object.
func (t *NetworkDevice) Clone() types.Target {
	return (*NetworkDevice)(proto.Clone(t.AsProto()).(*pb.NetworkDevice))
}

// NewFromFlags returns a Target configured from commandline flags.
func NewFromFlags(fl *Flags) (types.Target, error) {
	if fl.TargetType == "task" {
		if fl.TaskServiceName == "" {
			return nil, errors.New(
				"--ts-mon-task-service-name must be provided when using --ts-mon-target-type=task")
		}
		if fl.TaskJobName == "" {
			return nil, errors.New(
				"--ts-mon-task-job-name must be provided when using --ts-mon-target-type=task")
		}

		return (*Task)(&pb.Task{
			ServiceName: proto.String(fl.TaskServiceName),
			JobName:     proto.String(fl.TaskJobName),
			DataCenter:  proto.String(fl.TaskRegion),
			HostName:    proto.String(fl.TaskHostname),
			TaskNum:     proto.Int32(int32(fl.TaskNumber)),
		}), nil
	} else if fl.TargetType == "device" {
		return (*NetworkDevice)(&pb.NetworkDevice{
			Alertable: proto.Bool(true),
			Realm:     proto.String("ACQ_CHROME"),
			Metro:     proto.String(fl.DeviceRegion),
			Role:      proto.String(fl.DeviceRole),
			Hostname:  proto.String(fl.DeviceHostname),
			Hostgroup: proto.String(fl.DeviceNetwork),
		}), nil
	} else {
		return nil, fmt.Errorf("unknown --ts-mon-target-type '%s'", fl.TargetType)
	}
}
