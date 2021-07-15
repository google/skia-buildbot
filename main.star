#!/usr/bin/env lucicfg

luci.project(
    name = "Skia Buildbot",
    buildbucket = "cr-buildbucket.appspot.com",
    swarming = "chromium-swarm.appspot.com",
    acls = [
        acl.entry(acl.PROJECT_CONFIGS_READER, groups = [ "all" ]),
        acl.entry(acl.LOGDOG_READER, groups = [ "all" ]),
        acl.entry(acl.LOGDOG_WRITER, groups = [ "luci-logdog-skia-writers" ]),
        acl.entry(acl.CQ_COMMITTER, groups = [ "project-skia-committers" ]),
        acl.entry(acl.CQ_DRY_RUNNER, groups = [ "project-skia-tryjob-access" ]),
    ],
    logdog = "luci-logdog",
)

luci.logdog(
    gs_bucket = "skia-logdog",
)
