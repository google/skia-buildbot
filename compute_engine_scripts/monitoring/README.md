Monitoring (Graphite)
=====================

[Graphite](https://graphite.readthedocs.org/en/latest/) is a monitoring tool
for servers and services. We are using it to monitor the runtime performance
and behavior of the SkFiddle.com and the new SkPerf services, and maybe other
services in the future.

This document describes the setup procedure for the Graphite server and the
process for loading data into the server.

Full Server Setup
=================

Do once
-------

    $ ./vm_create_instance.sh
    $ ./vm_setup_instance.sh

Make sure to 'set daemon 2' in /etc/monit/monitrc so that monit
runs every 2 seconds.

Do on update
------------

    $ ./vm_push_update.sh

Notes
-----
To SSH into the instance:

    gcutil --project=google.com:skia-buildbots ssh --ssh_user=default skia-monitoring-b

If you need to modify the constants for the vm_XXX.sh scripts they are
specified in compute_engine_scripts/buildbot/vm_config.sh.
