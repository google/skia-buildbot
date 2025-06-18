package buildbucket

import (
	"go.skia.org/infra/mcp/common"
)

const (
	argBuilder   = "builder"
	argStartTime = "range_start_time"
	argEndTime   = "range_end_time"
	argMilestone = "milestone"
	argVersion   = "version"
	argBBID      = "bbid"
)

// GetTools returns tools supported by ReleaseInfra.
func GetTools(bbClient *BuildbucketClient) []common.Tool {
	return []common.Tool{
		{
			Name:        "get_best_revision_builds",
			Description: getBestRevisionBuildsDescription,
			Arguments:   []common.ToolArgument{},
			Handler:     bbClient.GetBestRevisionHandler,
		},
		// TODO(keybo@):Add a tool for getting milestone-to-branch mapping to
		// improve the versatility of the service.
		{
			Name:        "get_milestone_release_status",
			Description: getMilestoneReleaseStatusDescription,
			Arguments: []common.ToolArgument{
				{
					Name:        argMilestone,
					Description: "[Required] The Chrome milestine number as an integer.",
					Required:    true,
				},
			},
			Handler: bbClient.GetMilestoneStatusHandler,
		},
		// TODO(keybo@):Add a tool for getting released Chrome versions to
		// improve the versatility of the service.
		{
			Name:        "get_chrome_version_release_status",
			Description: getChromeVersionReleaseStatusDescription,
			Arguments: []common.ToolArgument{
				{
					Name:        argVersion,
					Description: "[Required] The Chrome version as a string.",
					Required:    true,
				},
			},
			Handler: bbClient.GetVersionStatusHandler,
		},
		{
			Name:        "get_build_steps",
			Description: getBuildStepsDescription,
			Arguments: []common.ToolArgument{
				{
					Name:        argBBID,
					Description: "[Required] The Buildbucket ID of a LUCI build",
					Required:    true,
				},
			},
			Handler: bbClient.GetBuildStepsHandler,
		},
		{
			Name:        "get_builds",
			Description: getBuildsDescription,
			Arguments: []common.ToolArgument{
				{
					Name:        argBuilder,
					Description: "[Required] Name of the LUCI builder name",
					Required:    true,
				},
				{
					Name:        argStartTime,
					Description: "[Optional] Start of the time range to search for builds, Epoch seconds as INT64.",
					Required:    false,
				},
				{
					Name:        argEndTime,
					Description: "[Optional] End of the time range to search for builds, Epoch seconds as INT64.",
					Required:    false,
				},
			},
			Handler: bbClient.GetBuildsHandler,
		},
	}
}
