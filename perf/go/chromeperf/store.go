package chromeperf

import (
	"context"
)

// ReverseKeyMapStore is the interface used to handle the updated param value in Skia.
type ReverseKeyMapStore interface {
	// Create a map from value and key to the original value in the db
	Create(ctx context.Context, modifiedValue string, key string, originalValue string) (string, error)

	// Get the original value by the value and key
	Get(ctx context.Context, modifiedValue string, key string) (string, error)
}
