#!/usr/bin/env lucicfg

# Enable LUCI Realms support.
lucicfg.enable_experiment("crbug.com/1085650")

luci.project(
    name = "Skia Buildbot",
    acls = [
        acl.entry(acl.PROJECT_CONFIGS_READER, groups = [ "all" ]),
        acl.entry(acl.LOGDOG_READER, groups = [ "all" ]),
        acl.entry(acl.LOGDOG_WRITER, groups = [ "luci-logdog-skia-writers" ]),
    ],
    logdog = "luci-logdog",
)

luci.logdog(
    gs_bucket = "skia-logdog",
)
