// Package prom has functions for Prometheus.
package prom

import (
	"regexp"
	"strings"
)

var (
	// atComparison is used to chop up expressions at a comparison. Note that we
	// require a trailing space, which avoids matching the equals sign inside
	// a label, such as {app="foo"}.
	atComparison = regexp.MustCompile(`[<>=!]+\s`)
)

// EquationFromExpr returns the equation from an expression. For example:
//
//      "liveness_ci_pubsub_receive_s > 60 * 60 * 24 * 2"
//
// Will return:
//
//       "liveness_ci_pubsub_receive_s"
//
// Note that for this to work the equation needs to be on the right hand side fo
// the expression, and there must be spaces on either side of any comparison
// operator.
//
// If an equation can't be extracted from the expression then false is returned.
func EquationFromExpr(expr string) (string, bool) {
	if expr == "" {
		return "", false
	}
	// Ignore computed metrics, which by convention have a ":".
	if strings.Contains(expr, ":") {
		return "", true
	}

	parts := atComparison.Split(expr, -1)
	// Ignore multipart relations, e.g. "a < b and b > c".
	if len(parts) != 2 {
		return "", true
	}

	ret := strings.TrimSpace(parts[0])
	if ret == "" {
		return "", false
	}

	return ret, false
}
