package slo

import (
	"fmt"
	"time"

	"github.com/hako/durafmt"
)

// Used by bugs central to determine if an issue has violated SLO. We only do SLOs for P0-P3.
// Uses data from https://docs.google.com/document/d/1OgpX1KDDq3YkHzRJjqRHSPJ9CJ8hH0RTvMAApKVxwm8/edit

const (
	// Convenient constants to use when calculating SLO violations.
	Daily     = 24 * time.Hour
	Weekly    = 7 * Daily
	Monthly   = 30 * Daily
	Biannualy = 6 * Monthly
	Yearly    = 2 * Biannualy
	Biennialy = 2 * Yearly
)

// // IsPrioritySLOViolation returns whether the priority is outside the SLO. This utility
// // function is in types package and not slo package because it would be a dependency cycle due to
// // types.StandardizedPriority.
// // If issue has violated SLO then returns description and a duration that shows by how much
// // it was surpassed.
// func IsPrioritySLOViolation(now, created, modified time.Time, priority StandardizedPriority) (bool, string, time.Duration) {
// 	switch priority {
// 	case PriorityP0:
// 		return slo.IsP0SLOViolation(now, created, modified)
// 	case PriorityP1:
// 		return slo.IsP1SLOViolation(now, created, modified)
// 	case PriorityP2:
// 		return slo.IsP2SLOViolation(now, created, modified)
// 	case PriorityP3:
// 		return slo.IsP3SLOViolation(now, created, modified)
// 	}
// 	return false, "", 0
// }

// IsP0SLOViolation is a utility function to determine if the P0 issue has violated SLO.
// If issue has violated SLO then returns description and a duration that shows by how much
// it was surpassed.
func IsP0SLOViolation(now, created, modified time.Time) (bool, string, time.Duration) {
	if now.After(modified.Add(Daily)) {
		duration := now.Sub(modified.Add(Daily))
		return true, fmt.Sprintf("exceeded modified time SLO by %s", durafmt.Parse(duration).LimitFirstN(2)), duration
	} else if now.After(created.Add(Weekly)) {
		duration := now.Sub(created.Add(Weekly))
		return true, fmt.Sprintf("exceeded creation time SLO by %s", durafmt.Parse(duration).LimitFirstN(2)), duration
	}
	return false, "", 0
}

// IsP1SLOViolation is a utility function to determine if the P1 issue has violated SLO.
// If issue has violated SLO then returns description and a duration that shows by how much
// it was surpassed.
func IsP1SLOViolation(now, created, modified time.Time) (bool, string, time.Duration) {
	if now.After(modified.Add(Weekly)) {
		duration := now.Sub(modified.Add(Weekly))
		return true, fmt.Sprintf("exceeded modified time SLO by %s", durafmt.Parse(duration).LimitFirstN(2)), duration
	} else if now.After(created.Add(Monthly)) {
		duration := now.Sub(created.Add(Monthly))
		return true, fmt.Sprintf("exceeded creation time SLO by %s", durafmt.Parse(duration).LimitFirstN(2)), duration
	}
	return false, "", 0
}

// IsP2SLOViolation is a utility function to determine if the P2 issue has violated SLO.
// If issue has violated SLO then returns description and a duration that shows by how much
// it was surpassed.
func IsP2SLOViolation(now, created, modified time.Time) (bool, string, time.Duration) {
	if now.After(modified.Add(Biannualy)) {
		duration := now.Sub(modified.Add(Biannualy))
		return true, fmt.Sprintf("exceeded modified time SLO by %s", durafmt.Parse(duration).LimitFirstN(2)), duration
	} else if now.After(created.Add(Yearly)) {
		duration := now.Sub(created.Add(Yearly))
		return true, fmt.Sprintf("exceeded creation time SLO by %s", durafmt.Parse(duration).LimitFirstN(2)), duration
	}
	return false, "", 0
}

// IsP3SLOViolation is a utility function to determine if the P3 issue has violated SLO.
// If issue has violated SLO then returns description and a duration that shows by how much
// it was surpassed.
func IsP3SLOViolation(now, created, modified time.Time) (bool, string, time.Duration) {
	if now.After(modified.Add(Yearly)) {
		duration := now.Sub(modified.Add(Yearly))
		return true, fmt.Sprintf("exceeded modified time SLO by %s", durafmt.Parse(duration).LimitFirstN(2)), duration
	} else if now.After(created.Add(Biennialy)) {
		duration := now.Sub(created.Add(Biennialy))
		return true, fmt.Sprintf("exceeded creation time SLO by %s", durafmt.Parse(duration).LimitFirstN(2)), duration
	}
	return false, "", 0
}
