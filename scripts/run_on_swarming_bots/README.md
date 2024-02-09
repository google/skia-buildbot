# Batch Running Custom Tasks On Swarming

Some times, we want to run a custom script on many swarming bots. Typically,
this is performing maintenance on a swarming bot, like updating/removing an
installed application or cleaning up disk space.

This go program, `run_on_swarming_bots`, makes that easy.

First, run the program in dry run mode to make sure you have your dimensions
correct and thus are running the script on the right set of bots. Protip: if you
want a single bot, you can specify it with the id dimension
(`id:skia-e-gce-123`).

    $ go run scripts/run_on_swarming_bots/run_on_swarming_bots.go \
        --dimension docker_installed:true --dimension gce:1 \
        --dry_run
    # output should include lines like:
    #   run_on_swarming_bots.go:135 Dry run mode.  Would run on following bots:
    #   run_on_swarming_bots.go:137 skia-e-gce-100
    #   run_on_swarming_bots.go:137 skia-e-gce-101
    #   run_on_swarming_bots.go:137 skia-e-gce-102
    # and then the program exits.

Finally, include your script. Your script will run on all swarming bots that
match the specified dimensions.

    $ go run scripts/run_on_swarming_bots/run_on_swarming_bots.go \
        --dimension docker_installed:true --dimension gce:1 \
        --script=scripts/run_on_swarming_bots/cleanup_docker.py

The program will print a link which you can use to track the status of the
tasks. If a subset of tasks fails (eg. bots didn't become available before the
tasks timed out), you can run the program again using the `--rerun` flag.
