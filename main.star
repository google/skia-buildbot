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

luci.cq (
    status_host = "chromium-cq-status.appspot.com",
    submit_burst_delay = time.duration(480000),
    submit_max_burst = 4,
)

luci.cq_group(
    name = "buildbot",
    watch = cq.refset(
        repo = "https://skia.googlesource.com/buildbot",
        refs = [ "refs/heads/.+" ],
    ),
    retry_config = cq.retry_config(
        single_quota = 0,
        global_quota = 0,
        failure_weight = 0,
        transient_failure_weight = 0,
        timeout_weight = 0,
    ),
    verifiers = [
        luci.cq_tryjob_verifier(
            builder = "skia:skia.primary/Housekeeper-OnDemand-Presubmit",
        ),
        luci.cq_tryjob_verifier(
            builder = "skia:skia.primary/Infra-PerCommit-Small",
        ),
        luci.cq_tryjob_verifier(
            builder = "skia:skia.primary/Infra-PerCommit-Medium",
        ),
        luci.cq_tryjob_verifier(
            builder = "skia:skia.primary/Infra-PerCommit-Large",
        ),
        luci.cq_tryjob_verifier(
            builder = "skia:skia.primary/Infra-PerCommit-Puppeteer",
        ),
        luci.cq_tryjob_verifier(
            builder = "skia:skia.primary/Infra-PerCommit-Build-Bazel-RBE",
        ),
        luci.cq_tryjob_verifier(
            builder = "skia:skia.primary/Infra-PerCommit-Test-Bazel-RBE",
        ),
        luci.cq_tryjob_verifier(
            builder = "skia:skia.primary/Infra-PerCommit-ValidateAutorollConfigs",
        ),
    ],
)
