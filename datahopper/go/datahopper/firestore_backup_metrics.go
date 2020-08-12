package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/metrics2"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/go/util"
	"golang.org/x/oauth2"
)

const (
	firestoreBackupOperationType = "type.googleapis.com/google.firestore.admin.v1.ExportDocumentsMetadata"
	firestoreBackupUriPrefix     = "gs://skia-firestore-backup/everything/"
)

// firestoreOperations decodes the JSON response from a Firestore list operations request.
type firestoreOperations struct {
	Operations    []firestoreOperation `json:"operations"`
	NextPageToken string               `json:"nextPageToken"`
}

// firestoreOperation partially decodes a single Firestore operation from JSON.
type firestoreOperation struct {
	Name     string                     `json:"name"`
	Metadata firestoreOperationMetadata `json:"metadata"`
	Done     bool                       `json:"done"`
	// other fields omitted
}

// firestoreOperationMetadata is a sub-struct of firestoreOperation.
type firestoreOperationMetadata struct {
	Type            string `json:"@type"`
	EndTime         string `json:"endTime"`
	OperationState  string `json:"operationState"`
	OutputUriPrefix string `json:"outputUriPrefix"`
	// other fields omitted
}

// Returns the most recent time that a Firestore ExportDocumentsMetadata operation exporting to
// firestoreBackupUriPrefix completed successfully.
func getFirestoreLastBackupCompleted(ctx context.Context, httpClient *http.Client) (time.Time, error) {
	z := time.Time{}
	latest := time.Time{}
	nextPageToken := ""
	for {
		listURL := fmt.Sprintf("https://firestore.googleapis.com/v1/projects/skia-firestore/databases/(default)/operations?filter=%s", url.QueryEscape("metadata.operationState=SUCCESSFUL"))
		if nextPageToken != "" {
			listURL = listURL + "&pageToken=" + nextPageToken
		}
		sklog.Debugf("Sending Firestore list operations request: %q", listURL)
		resp, err := httpClient.Get(listURL)
		if err != nil {
			return z, fmt.Errorf("Error performing Firestore list operations request: %s", err)
		}
		operations := firestoreOperations{}
		if err := json.NewDecoder(resp.Body).Decode(&operations); err != nil {
			return z, fmt.Errorf("Unable to decode Firestore list operations response as JSON: %s", err)
		}
		sklog.Debugf("Decoded Firestore list operations response: %+v", operations)
		for _, o := range operations.Operations {
			if o.Done &&
				o.Metadata.Type == firestoreBackupOperationType &&
				o.Metadata.OperationState == "SUCCESSFUL" &&
				strings.HasPrefix(o.Metadata.OutputUriPrefix, firestoreBackupUriPrefix) {
				if parsed, err := time.Parse(time.RFC3339Nano, o.Metadata.EndTime); err != nil {
					return z, fmt.Errorf("Invalid time %q in Firestore list operations response for %q: %s", o.Metadata.EndTime, o.Name, err)
				} else if parsed.After(latest) {
					latest = parsed
				}
			}
		}
		if operations.NextPageToken == "" {
			break
		} else {
			nextPageToken = operations.NextPageToken
		}
	}
	if util.TimeIsZero(latest) {
		return z, fmt.Errorf("Firestore list operations response contained no matching results.")
	}
	return latest, nil
}

// StartFirestoreBackupMetrics starts a goroutine to periodically update the
// last_successful_firestore_backup liveness metric.
func StartFirestoreBackupMetrics(ctx context.Context, tokenSource oauth2.TokenSource) error {
	httpClient := httputils.DefaultClientConfig().WithTokenSource(tokenSource).With2xxOnly().Client()
	lvMetrics := metrics2.NewLiveness("last_successful_firestore_backup_metrics_update")
	lvBackup := metrics2.NewLiveness("last_successful_firestore_backup")
	go util.RepeatCtx(ctx, 5*time.Minute, func(ctx context.Context) {
		if lastBackupTime, err := getFirestoreLastBackupCompleted(ctx, httpClient); err != nil {
			sklog.Errorf("Failed to update firestore backup metrics: %s", err)
		} else {
			lvBackup.ManualReset(lastBackupTime)
			lvMetrics.Reset()
		}
	})
	return nil
}
