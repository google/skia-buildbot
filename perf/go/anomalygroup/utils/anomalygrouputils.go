package utils

import (
	"context"
	"strings"
	"sync"
	"time"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/perf/go/alerts"
	ag "go.skia.org/infra/perf/go/anomalygroup/proto/v1"
	backend "go.skia.org/infra/perf/go/backend/client"
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
	resp, err := ag_client.FindExistingGroups(
		ctx,
		&ag.FindExistingGroupsRequest{
			// Subscription info will be loaded from alerts.Alert in the future.
			// Using hard coded values for now. Subscription name will diff from day to day.
			SubscriptionName:     "Test Sub Name",
			SubscriptionRevision: "Test Sub Rev " + time.Now().Format("2000-01-01"),
			Action:               ag.GroupActionType(ag.GroupActionType_value[groupAction]),
			StartCommit:          startCommit,
			EndCommit:            endCommit,
			TestPath:             testPath})
	if err != nil {
		return "", skerr.Wrapf(err, "error finding existing group for new anomaly")
	}

	if len(resp.AnomalyGroups) == 0 {
		// No existing group is found -> create a new group
		newGroupID, err := ag_client.CreateNewAnomalyGroup(
			ctx,
			&ag.CreateNewAnomalyGroupRequest{
				SubscriptionName:     "Test Sub Name",
				SubscriptionRevision: "Test Sub Rev " + time.Now().Format("2000-01-01"),
				Domain:               paramSet["master"],
				Benchmark:            paramSet["benchmark"],
				StartCommit:          startCommit,
				EndCommit:            endCommit,
				Action:               ag.GroupActionType(ag.GroupActionType_value[groupAction])})
		if err != nil {
			return "", skerr.Wrapf(err, "error finding existing group for new anomaly")
		}
		sklog.Info("Created new anomaly group: %s for anomaly %s", newGroupID, anomalyID)
		// TODO(wenbinzhang): Update on create in one step.
		_, err = ag_client.UpdateAnomalyGroup(
			ctx,
			&ag.UpdateAnomalyGroupRequest{
				AnomalyGroupId: newGroupID.AnomalyGroupId,
				AnomalyId:      anomalyID})
		if err != nil {
			return "", skerr.Wrapf(err, "error updating group with new anomaly")
		}
		// TODO(wenbinzhang:b/329900854): trigger temporal workflow with new group id
	} else {
		sklog.Info("Found %d existing anomaly groups for anomaly %s", len(resp.AnomalyGroups), anomalyID)
		// found matching groups for the new anomaly
		for _, alertGroup := range resp.AnomalyGroups {
			groupID := alertGroup.GroupId
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

	groupIDs := []string{}
	for _, group := range resp.AnomalyGroups {
		groupIDs = append(groupIDs, group.GroupId)
	}
	return strings.Join(groupIDs, ","), nil
}
