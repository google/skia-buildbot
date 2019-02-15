package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.skia.org/infra/go/exec"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
)

const (
	firestoreBackupUriPrefix = "gs://skia-firestore-backup/"
)

// Returns the most recent time that a firestore operation to save data to firestoreBackupUriPrefix
// completed succesfully.
func getFirestoreLastBackupCompleted(ctx context.Context) (time.Time, error) {
	// Get a filtered list of firestore operations and output the endTime.
	out, err := exec.RunCommand(ctx, &exec.Command{
		Name: "gcloud",
		Args: []string{
			"beta", "firestore", "operations", "list",
			"--project=skia-firestore",
			fmt.Sprintf("--filter=metadata.outputUriPrefix~^%s AND metadata.operationState=SUCCESSFUL", firestoreBackupUriPrefix),
			"--format=value(metadata.endTime)",
		},
	})
	if err != nil {
		return time.Time{}, err
	}
	// You'd think we could use "--sort-by=~metadata.endTime" and "--limit=1" in the above command,
	// but those flags are buggy.
	latest := time.Time{}
	for _, line := range strings.Split(out, "\n") {
		if line == "" {
			continue
		}
		if parsed, err := time.Parse(time.RFC3339Nano, line); err != nil {
			return time.Time{}, err
		} else if parsed.After(latest) {
			latest = parsed
		}
	}
	return latest, nil
}

// StartFirestoreBackupMetrics starts a goroutine to periodically update the
// last_successful_firestore_backup liveness metric.
func StartFirestoreBackupMetrics(ctx context.Context) error {
	lvMetrics := metrics2.NewLiveness("last_successful_firestore_backup_metrics_update")
	lvBackup := metrics2.NewLiveness("last_successful_firestore_backup")
	go util.RepeatCtx(5*time.Minute, ctx, func() {
		if lastBackupTime, err := getFirestoreLastBackupCompleted(ctx); err != nil {
			sklog.Errorf("Failed to update firestore backup metrics: %s", err)
		} else {
			lvBackup.ManualReset(lastBackupTime)
			lvMetrics.Reset()
		}
	})
	return nil
}
