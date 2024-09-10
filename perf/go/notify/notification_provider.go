package notify

import (
	"context"

	"go.skia.org/infra/perf/go/notify/common"
)

// NotificationDataProvider provides an interface to be used to perform notification data related operations.
type NotificationDataProvider interface {
	// GetNotificationDataRegressionFound returns a notificationData object for the given regression metadata.
	GetNotificationDataRegressionFound(context.Context, common.RegressionMetadata) (*common.NotificationData, error)

	// GetNotificationDataRegressionMissing returns a notificationData object for the given regression metadata.
	GetNotificationDataRegressionMissing(context.Context, common.RegressionMetadata) (*common.NotificationData, error)
}

type defaultNotificationDataProvider struct {
	formatter Formatter
}

func (prov *defaultNotificationDataProvider) GetNotificationDataRegressionFound(ctx context.Context, metadata common.RegressionMetadata) (*common.NotificationData, error) {
	body, subject, err := prov.formatter.FormatNewRegression(
		ctx,
		metadata.CurrentCommit,
		metadata.PreviousCommit,
		metadata.AlertConfig,
		metadata.Cl,
		metadata.InstanceUrl,
		metadata.Frame)
	if err != nil {
		return nil, err
	}

	return &common.NotificationData{
		Body:    body,
		Subject: subject,
	}, nil
}

func (prov *defaultNotificationDataProvider) GetNotificationDataRegressionMissing(ctx context.Context, metadata common.RegressionMetadata) (*common.NotificationData, error) {
	return prov.GetNotificationDataRegressionFound(ctx, metadata)
}

// newDefaultNotificationProvider returns a new instance of the defaultNotificationDataProvider.
func newDefaultNotificationProvider(formatter Formatter) *defaultNotificationDataProvider {
	return &defaultNotificationDataProvider{
		formatter: formatter,
	}
}
