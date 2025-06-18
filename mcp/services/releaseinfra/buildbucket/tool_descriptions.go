package buildbucket

const getBestRevisionBuildsDescription = `
List the most recent builds of the chrome-best-revision-continuous builder.
The "best revision" is a commit in the chromium/src repo and is used as
the branching point for Chrome related repositories. It is provided in
the output properties of successful builds from the builder, including
the commit hash, commit position and age. The most recent best revision
is the next branching point. This tool fetches the most recent 8 builds
only. With each build determines the best revision for the past 4 hours,
8 builds cover the past 12 hours, which is the gap between each Chrome
branching. No successful builds means that there were no qualified best
revision in the past 12 hours. Use this tool when user asks about
branching point or branching commit for Chrome.
`

const getMilestoneReleaseStatusDescription = `
List the build status of all platforms for a specific Chrome milestone number.
Use this tool when user asks about Chrome release of a specific milestone
number. The milestone number is an integer greater 0. Sometimes user
may refer to a milestone with an "M" in front, such as "M123".
`

const getChromeVersionReleaseStatusDescription = `
List the build status of all platforms for a specific Chrome version. Use this
tool when user asks about Chrome release of a specific Chrome version. The
Chrome version is 4 integers separated by ".", for example "1.2.3.4".
`

const getBuildStepsDescription = `
List the build steps of a Chrome build. The list of steps provide details about
the execution and the status of each step. This is very useful when the user
asks about the failure that happened in a build. The Buildbucket ID input is a
non-zero integer, which is commonly at the end of a build URL, such as
"8712076428762752673" in
"https://cr-buildbucket.appspot.com/build/8712076428762752673".
`

const getBuildsDescription = `
List Chrome official builds by builder name and range of timestamps. Use this
tool when the user wants to find the builds for a specific builder. If no time
range is provided, this tool returns the most recent 10 builds only.
`
