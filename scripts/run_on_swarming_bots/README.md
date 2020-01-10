Batch Running Custom Tasks On Swarming
======================================

Some times, we want to run a custom script on many swarming bots.
Typically, this is a maintenance task, like updating/removing
an installed application or cleaning up disk space.

This go program, `run_on_swarming_bots`, makes that easy.

First, make sure you have `isolate` and `isolated` downloaded
from CIPD and on your PATH somewhere. The cipd executable
comes in depot_tools, so make sure that is set up before proceding.

    # All code examples are run from the infra repo's root
    cipd ensure --root=$HOME/bin/luci --ensure-file=cipd.ensure
    export PATH=$PATH:$HOME/bin/luci/cipd_bin_packages

Then, run the program in dry run mode to make sure you have
your dimensions correct.

    go run scripts/run_on_swarming_bots/run_on_swarming_bots.go --logtostderr \
    --dimension docker_installed:true --dimension gce:1 \
    --dry_run

Finally, include your script. Assuming things work out, it's good
to check in this script if would be useful in the future.

    go run scripts/run_on_swarming_bots/run_on_swarming_bots.go --logtostderr \
    --dimension docker_installed:true --dimension gce:1 \
    --script=scripts/run_on_swarming_bot/cleanup_docker.py