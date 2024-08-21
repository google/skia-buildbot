package workflows

import (
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
)

// Workflow name definitions.
//
// Those are used to invoke the workflows. This is meant to decouple the
// souce code dependencies such that the client doesn't need to link with
// the actual implementation.
// TODO(b/326352379): introduce a specific type to encapsulate these workflow names
const (
	ProcessCulprit        = "perf.process_culprit"
	MaybeTriggerBisection = "perf.maybe_trigger_bisection"
)

type ProcessCulpritParam struct {
	CulpritServiceUrl string
	Commits           []*pinpoint_proto.Commit
	AnomalyGroupId    string
}

type ProcessCulpritResult struct {
	CulpritIds []string
	IssueIds   []string
}

type MaybeTriggerBisectionParam struct {
	AnomalyGroupServiceUrl string
	CulpritServiceUrl      string
	AnomalyGroupId         string
	GroupingTaskQueue      string
	PinpointTaskQueue      string
}

type MaybeTriggerBisectionResult struct {
	JobId string
}
