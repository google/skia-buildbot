// Copyright 2026 Google LLC
//
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package alerts

import (
	"fmt"

	"go.skia.org/infra/am/go/incident"
	"go.skia.org/infra/mcp/common"
	"go.skia.org/infra/mcp/services/skia/format"
)

const (
	argCategory        = incident.CATEGORY
	argSeverity        = incident.SEVERITY
	argAlertName       = incident.ALERT_NAME
	argAssignedTo      = incident.ASSIGNED_TO
	argOwner           = incident.OWNER
	argIncludeSilenced = "include_silenced"
	argIncludeAssigned = "include_assigned"
	argAlertID         = "alert_id"
	argHistoryLimit    = "history_limit"
)

func GetTools(c *AlertsClient) []common.Tool {
	return []common.Tool{
		{
			Name:        "get_active_alerts",
			Description: "Retrieve active alerts from Alert Manager (am.skia.org).",
			Arguments: []common.ToolArgument{
				{
					Name:        argCategory,
					Description: `[Optional] Filter alerts by category, eg. "infra" or "skia". If not provided, alerts from all categories are returned.`,
				},
				{
					Name:        argSeverity,
					Description: `[Optional] Filter alerts by severity, eg. "critical" or "warning".`,
				},
				{
					Name:        argAlertName,
					Description: `[Optional] Filter alerts by alertname, eg. "JobDown" or "BotMissing".`,
				},
				{
					Name:        argAssignedTo,
					Description: `[Optional] Filter alerts explicitly assigned to a specific person, eg. "user@google.com".`,
				},
				{
					Name:        argOwner,
					Description: `[Optional] Filter alerts by owner, eg. "user@google.com".`,
				},
				{
					Name:         argIncludeSilenced,
					Description:  "[Optional] If true, include alerts that match an active silence. Default: false.",
					ArgumentType: common.BooleanArgument,
				},
				{
					Name:         argIncludeAssigned,
					Description:  "[Optional] If true, include alerts that are already assigned to a user. Default: true.",
					ArgumentType: common.BooleanArgument,
				},
				format.FormatToolArgument(),
			},
			Handler: format.FormatResponseWrapper(c.GetActiveAlertsHandler),
		},
		{
			Name:        "get_alert_details",
			Description: "Retrieve full details, matching silences, flakiness analysis, and recent history for a single alert.",
			Arguments: []common.ToolArgument{
				{
					Name:        argAlertID,
					Description: "ID of the alert.",
					Required:    true,
				},
				{
					Name:         argHistoryLimit,
					Description:  fmt.Sprintf("[Optional] Maximum number of historical resolved occurrences to include. Default: %d.", incident.NUM_RECENTLY_RESOLVED_FOR_ID),
					ArgumentType: common.NumberArgument,
				},
				format.FormatToolArgument(),
			},
			Handler: format.FormatResponseWrapper(c.GetAlertDetailsHandler),
		},
	}
}
