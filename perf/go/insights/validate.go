// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package insights

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

var (
	// uuidRE matches canonical, lower-case UUIDs: the format of Regressions2, AnomalyGroups and
	// Culprits IDs.
	uuidRE = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

	// gitHashRE matches a full git commit hash.
	gitHashRE = regexp.MustCompile(`^[0-9a-f]{40}$`)

	// commitNumberRE matches a commit number (commit position).
	commitNumberRE = regexp.MustCompile(`^[0-9]{1,10}$`)
)

// maxNameLength is the maximum length of names (sheriff, benchmark, bot) accepted as input.
const maxNameLength = 256

const idHint = "ids are UUIDs as returned by list_anomalies, e.g. 6b1c2d3e-0000-4000-8000-000000000000"

// isValidID returns true if id is a canonical, lower-case UUID.
func isValidID(id string) bool {
	return uuidRE.MatchString(id)
}

// normalizeIDs trims, lower-cases, de-duplicates and strictly validates IDs, preserving order.
//
// SECURITY: this is the guard for Gate G1. sqlregression2store.GetByIDs still interpolates IDs
// into its SQL statement (FIX-01 has not landed), so every ID passed to it MUST be a canonical
// UUID. Never call GetByIDs with IDs that did not go through this function (or isValidID).
// Invalid input is reported by position only, so untrusted input is not echoed back.
func normalizeIDs(field string, ids []string, max int) ([]string, error) {
	if len(ids) == 0 {
		return nil, InvalidArgument("pass at least one id, e.g. from list_anomalies", "%s must not be empty", field)
	}
	if len(ids) > max {
		return nil, InvalidArgument(
			fmt.Sprintf("split the request into batches of at most %d ids", max),
			"too many %s: %d (max %d)", field, len(ids), max)
	}
	seen := make(map[string]bool, len(ids))
	ret := make([]string, 0, len(ids))
	for i, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if !isValidID(id) {
			if len(ids) == 1 {
				return nil, InvalidArgument(idHint, "%s is not a valid id", field)
			}
			return nil, InvalidArgument(idHint, "%s[%d] is not a valid id", field, i)
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		ret = append(ret, id)
	}
	return ret, nil
}

// validIDs returns the IDs that are canonical UUIDs, dropping the others. It is used for IDs that
// come from the database (e.g. AnomalyGroups.anomaly_ids) before they are passed to GetByIDs.
func validIDs(ids []string) []string {
	ret := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.ToLower(strings.TrimSpace(id))
		if isValidID(id) {
			ret = append(ret, id)
		}
	}
	return ret
}

// validateName checks that a name (sheriff, benchmark, bot) is non-empty, not too long and free
// of control characters.
func validateName(field, value string) error {
	if value == "" {
		return InvalidArgument("", "%s must not be empty", field)
	}
	if len(value) > maxNameLength {
		return InvalidArgument("", "%s is longer than %d characters", field, maxNameLength)
	}
	for _, r := range value {
		if unicode.IsControl(r) {
			return InvalidArgument("", "%s contains control characters", field)
		}
	}
	return nil
}

// validateOptionalName is validateName for optional filters.
func validateOptionalName(field, value string) error {
	if value == "" {
		return nil
	}
	return validateName(field, value)
}

// normalizeEnum lower-cases v and checks that it is one of allowed. An empty value maps to def.
func normalizeEnum(field, v, def string, allowed ...string) (string, error) {
	v = strings.ToLower(strings.TrimSpace(v))
	if v == "" {
		return def, nil
	}
	for _, a := range allowed {
		if v == a {
			return v, nil
		}
	}
	return "", InvalidArgument(fmt.Sprintf("use one of: %s", strings.Join(allowed, ", ")), "invalid %s", field)
}

// normalizeDays applies the default and bounds for a days filter.
func normalizeDays(days int) (int, error) {
	if days == 0 {
		return DefaultDays, nil
	}
	if days < 1 || days > MaxDays {
		return 0, InvalidArgument(fmt.Sprintf("use a value between 1 and %d", MaxDays), "invalid days: %d", days)
	}
	return days, nil
}

// normalizePageSize applies the default and bounds for a page size.
func normalizePageSize(n int) (int, error) {
	if n == 0 {
		return DefaultPageSize, nil
	}
	if n < 1 || n > MaxPageSize {
		return 0, InvalidArgument(fmt.Sprintf("use a value between 1 and %d", MaxPageSize), "invalid page_size: %d", n)
	}
	return n, nil
}

// truncate shortens s to at most n bytes, for safe inclusion in messages.
func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
