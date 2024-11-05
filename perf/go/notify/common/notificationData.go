package common

import (
	"go.skia.org/infra/perf/go/alerts"
	"go.skia.org/infra/perf/go/clustering2"
	"go.skia.org/infra/perf/go/git/provider"
	"go.skia.org/infra/perf/go/ui/frame"
)

// NotificationData provides a struct to contain data to be used for regression notifications.
type NotificationData struct {
	// The body of the notification.
	Body string
	// The subject of the notification.
	Subject string
}

// RegressionMetadata provides a struct to hold metadata related to the regression for notification generation.
type RegressionMetadata struct {
	RegressionCommit provider.Commit
	PreviousCommit   provider.Commit
	AlertConfig      *alerts.Alert
	Cl               *clustering2.ClusterSummary
	Frame            *frame.FrameResponse
	InstanceUrl      string

	// The fields below are only available when detection mode is Individual
	RegressionCommitLinks map[string]string
	PreviousCommitLinks   map[string]string
	TraceID               string
}
