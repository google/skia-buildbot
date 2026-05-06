package provider

import "context"

// Provider defines the interface for a visibility source.
type Provider interface {
	// GetExpectedRules fetches and returns the list of expected rules.
	GetExpectedRules(ctx context.Context) (map[string]bool, error)
}
