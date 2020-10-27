package slo

import (
	"time"
)

const (
	// Convenient constants to use when calculating SLO violations.
	Daily     = 24 * time.Hour
	Weekly    = 7 * Daily
	Monthly   = 30 * Daily
	Biannualy = 6 * Monthly
	Yearly    = 2 * Biannualy
	Biennialy = 2 * Yearly
)

// Used by bugs central to determine if an issue has violated SLO. We only do SLOs for P0-P3.
// Uses data from https://docs.google.com/document/d/1OgpX1KDDq3YkHzRJjqRHSPJ9CJ8hH0RTvMAApKVxwm8/edit

// IsP0SLOViolation is a utility function to determine if the P0 issue has violated SLO.
func IsP0SLOViolation(now, created, modified time.Time) bool {
	return now.After(modified.Add(Daily)) || now.After(created.Add(Weekly))
}

// IsP1SLOViolation is a utility function to determine if the P1 issue has violated SLO.
func IsP1SLOViolation(now, created, modified time.Time) bool {
	return now.After(modified.Add(Weekly)) || now.After(created.Add(Monthly))
}

// IsP2SLOViolation is a utility function to determine if the P2 issue has violated SLO.
func IsP2SLOViolation(now, created, modified time.Time) bool {
	return now.After(modified.Add(Biannualy)) || now.After(created.Add(Yearly))
}

// IsP3SLOViolation is a utility function to determine if the P3 issue has violated SLO.
func IsP3SLOViolation(now, created, modified time.Time) bool {
	return now.After(modified.Add(Yearly)) || now.After(created.Add(Biennialy))
}
