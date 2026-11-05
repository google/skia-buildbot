// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package schema

// PublicTraceRulesSchema represents the PublicTraceRules table in the database.
type PublicTraceRulesSchema struct {
	RuleExpression string `sql:"public_rule_expr TEXT PRIMARY KEY"`
}
