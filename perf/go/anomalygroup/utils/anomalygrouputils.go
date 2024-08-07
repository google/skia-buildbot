package utils

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.temporal.io/sdk/client"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	ag "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	backend "go.skia.org/infra/perf/go/backend/client"
	"go.skia.org/infra/perf/go/config"
	"go.skia.org/infra/perf/go/workflows"
	tpr_client "go.skia.org/infra/temporal/go/client"
	"go.temporal.io/sdk/temporal"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

var groupingMutex sync.Mutex

type AnomalyGrouper interface {
	ProcessRegressionInGroup(ctx context.Context, alert *alerts.Alert, anomalyID string, startCommit int64, endCommit int64, testPath string, paramSet map[string]string) (string, error) //
}

type AnomalyGrouperImpl struct{}

// implementation of ProcessRegressionInGroup for the AnomalyGrouper interface.
func (a *AnomalyGrouperImpl) ProcessRegressionInGroup(
	ctx context.Context, alert *alerts.Alert, anomalyID string, startCommit int64, endCommit int64, testPath string, paramSet map[string]string) (string, error) {
	return ProcessRegression(ctx, alert, anomalyID, startCommit, endCommit, testPath, paramSet)
}

// Process the regression with the following steps:
//  1. find an existing group if any, otherwise create a new group.
//  2. Update the existing issue, if any, with the regression's info.
func ProcessRegression(
	ctx context.Context,
	alert *alerts.Alert,
	anomalyID string,
	startCommit int64,
	endCommit int64,
	testPath string,
	paramSet map[string]string) (string, error) {
	// TODO(wenbinzhang): We need to process one regression at a time to avoid
	// race on creating new groups. However, multiple containers are created to
	// process regressions in parallel. We need to update the mutex usage here.
	groupingMutex.Lock()
	defer groupingMutex.Unlock()

	ag_client, err := backend.NewAnomalyGroupServiceClient("", false)
	if err != nil {
		return "", skerr.Wrapf(err, "error creating anomaly group client from backend")
	}

	groupAction := strings.ToUpper(string(alert.Action))
	sklog.Debugf(
		"Looking for groups for regression. SubName: %s, SubRev: %s, Action: %s, Start: %s, End: %s, Path: %s",
		alert.SubscriptionName, alert.SubscriptionRevision, alert.Action, startCommit, endCommit, testPath)
	resp, err := ag_client.FindExistingGroups(
		ctx,
		&ag.FindExistingGroupsRequest{
			// Subscription info will be loaded from alerts.Alert in the future.
			// Using hard coded values for now. Subscription name will diff from day to day.
			SubscriptionName:     alert.SubscriptionName,
			SubscriptionRevision: alert.SubscriptionRevision,
			Action:               ag.GroupActionType(ag.GroupActionType_value[groupAction]),
			StartCommit:          startCommit,
			EndCommit:            endCommit,
			TestPath:             testPath})
	if err != nil {
		return "", skerr.Wrapf(err, "error finding existing group for new anomaly")
	}

	groupIDs := []string{}
	if len(resp.AnomalyGroups) == 0 {
		// No existing group is found -> create a new group
		newGroupID, err := ag_client.CreateNewAnomalyGroup(
			ctx,
			&ag.CreateNewAnomalyGroupRequest{
				SubscriptionName:     alert.SubscriptionName,
				SubscriptionRevision: alert.SubscriptionRevision,
				Domain:               paramSet["master"],
				Benchmark:            paramSet["benchmark"],
				StartCommit:          startCommit,
				EndCommit:            endCommit,
				Action:               ag.GroupActionType(ag.GroupActionType_value[groupAction])})
		if err != nil {
			return "", skerr.Wrapf(err, "error finding existing group for new anomaly")
		}
		sklog.Info("Created new anomaly group: %s for anomaly %s", newGroupID, anomalyID)
		groupIDs = append(groupIDs, newGroupID.AnomalyGroupId)

		// TODO(wenbinzhang): Update on create in one step.
		_, err = ag_client.UpdateAnomalyGroup(
			ctx,
			&ag.UpdateAnomalyGroupRequest{
				AnomalyGroupId: newGroupID.AnomalyGroupId,
				AnomalyId:      anomalyID})
		if err != nil {
			return "", skerr.Wrapf(err, "error updating group with new anomaly")
		}

		// a MVP implementation of the temporal workflow invoke.
		temporalProvider := tpr_client.DefaultTemporalProvider{}
		temporalClient, cleanup, err := temporalProvider.NewClient(
			config.Config.TemporalConfig.HostPort, config.Config.TemporalConfig.Namespace)
		if err != nil {
			return "", skerr.Wrapf(err, "Error creating temporal client.")
		}
		defer cleanup()

		wo := client.StartWorkflowOptions{
			TaskQueue: config.Config.TemporalConfig.GroupingTaskQueue,
			// 30 minutes wait + handling time
			WorkflowExecutionTimeout: 2 * time.Hour,
			RetryPolicy: &temporal.RetryPolicy{
				// We will only attempt to run the workflow exactly once as we expect any failure will be
				// not retry-recoverable failure.
				MaximumAttempts: 1,
			},
		}
		// TODO(wenbinzhang): the anomaly group service url and the culprit service url are actually the backend
		//   service host url override. We should rename and use one single property.
		wf, err := temporalClient.ExecuteWorkflow(
			ctx, wo, workflows.MaybeTriggerBisection, &workflows.MaybeTriggerBisectionParam{
				AnomalyGroupServiceUrl: config.Config.BackendServiceHostUrl,
				CulpritServiceUrl:      config.Config.BackendServiceHostUrl,
				AnomalyGroupId:         newGroupID.AnomalyGroupId,
				GroupingTaskQueue:      config.Config.TemporalConfig.GroupingTaskQueue,
				PinpointTaskQueue:      config.Config.TemporalConfig.PinpointTaskQueue,
			})
		if err != nil {
			return "", status.Errorf(codes.Internal, "Unable to start grouping workflow (%v).", err)
		}
		sklog.Infof("Grouping workflow created: %s", wf.GetID())
	} else {
		sklog.Info("Found %d existing anomaly groups for anomaly %s", len(resp.AnomalyGroups), anomalyID)
		// found matching groups for the new anomaly
		for _, alertGroup := range resp.AnomalyGroups {
			groupID := alertGroup.GroupId
			groupIDs = append(groupIDs, groupID)
			_, err = ag_client.UpdateAnomalyGroup(
				ctx,
				&ag.UpdateAnomalyGroupRequest{
					AnomalyGroupId: groupID,
					AnomalyId:      anomalyID})
			if err != nil {
				return "", skerr.Wrapf(err, "error updating group with new anomaly")
			}
		}
		// TODO(wenbinzhang:b/329900854): take actions based on alertGroup.GroupAction
	}

	return strings.Join(groupIDs, ","), nil
}
