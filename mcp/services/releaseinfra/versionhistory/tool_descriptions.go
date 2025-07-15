package versionhistory

const listChromePlatformsDescription = `
Get a list of platform which the Chrome browser has been released
for. Each entry in the list includes a "name" field in the format
of "chrome/platforms/{platform_name}", and those "platform_name"
strings are used in the requests and responses of other tools
provided by this service.
`

const listChromeChannelsDescription = `
Get a list of channels of Chrome browser releases. Each entry in
the list includes a "name" field in the format of
"chrome/platforms/{platform_name}/channels/{channel_name}", and
the "channel_name" strings are used in the requests and responses
of other tools provided by this service. If a platform name is
provided, the response will only include supported channels for
the provided platform. Otherwise, the response will include
supported channels for all platforms.
`

const listActiveReleasesDescription = `
Get a list of version information of the Chrome browser that are
currently being actively served to the users. This list includes
versions for all channels and across all platforms.
`

const listReleaseInfoDescription = `
Get a list of version information of the Chrome browser that has
been released. Each version info in the list contains the
following fields:
- name: this contains the platform, channel, version of the
  released Chrome browser.
- serving: the start time and end time of this version. A version
  without an end time field means that this version is still being
  served to users.
- fraction: the proportion of the user population that this
  version is being released to.
- version: the Chrome browser version released.

The version info list is sorted in descending order by the
serving start time of the version.

If the user does not specify a time range, the default time range
should be:
**End Time: Current date/time; Start Time: Current date/time minus 15 days.**

This tool also supports filtering Chrome releases by any
combination of the platforms, channel and version. If a filter is
not provided, then the response will include all possible values
of that filter. If no filters are provided, the response will
include all Chrome versions that are released in all channels
across all platforms during the given time range.

The list is created based on available real-time data from the
Version History API.
`
