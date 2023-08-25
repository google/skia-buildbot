// Package alert group contains code to interact with alert group apis on chromeperf
package alertgroup

import (
	"context"
)

// Service provides the interface to interact with AlertGroup apis on chromeperf
type Service interface {
	GetAlertGroupDetails(ctx context.Context, groupKey string) (*AlertGroupDetails, error)
}
