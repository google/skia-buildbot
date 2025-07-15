package versionhistory

import (
	"go.skia.org/infra/mcp/common"
)

const (
	argStarttime = "filter_start_time"
	argEndtime   = "filter_end_time"
	argPlatform  = "platform"
	argChannel   = "channel"
	argVersion   = "version"
)

// GetTools returns tools supported by ReleaseInfra.
func GetTools(c *VersionHistoryClient) []common.Tool {
	return []common.Tool{
		{
			Name:        "list_chrome_platforms",
			Description: listChromePlatformsDescription,
			Arguments:   []common.ToolArgument{},
			Handler:     c.GetChromePlatformsHandler,
		},
		{
			Name:        "list_chrome_channels",
			Description: listChromeChannelsDescription,
			Arguments: []common.ToolArgument{
				{
					Name:        argPlatform,
					Description: `[Optional] The platform name.`,
					Required:    false,
				},
			},
			Handler: c.GetChromeChannelsHandler,
		},
		{
			Name:        "list_active_releases",
			Description: listActiveReleasesDescription,
			Arguments:   []common.ToolArgument{},
			Handler:     c.GetActiveReleasesHandler,
		},
		{
			Name:        "list_release_info",
			Description: listReleaseInfoDescription,
			Arguments: []common.ToolArgument{
				{
					Name: argStarttime,
					Description: `
[Required] The start of the time range to search for a Chrome release.
The input should be in the RFC 3339 format and the Pacific timezone should be
used ad the default timezone, eg. "2025-07-12T14:30:00-07:00".`,
					Required: true,
				},
				{
					Name: argEndtime,
					Description: `
[Required] The end of the time range to search for a Chrome release.
The input should be in the RFC 3339 format and the Pacific timezone should be
used ad the default timezone, eg. "2025-07-12T14:30:00-07:00".`,
					Required: true,
				},
				{
					Name:        argPlatform,
					Description: `[Optional] The platform name.`,
					Required:    false,
				},
				{
					Name:        argChannel,
					Description: `[Optional] The channel name.`,
					Required:    false,
				},
				{
					Name:        argVersion,
					Description: `[Optional] The Chrome version string, eg. "1.2.3.4".`,
					Required:    false,
				},
			},
			Handler: c.GetVersionInfoHandler,
		},
	}
}
