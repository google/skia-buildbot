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

After the setup has completed once, do the following

    sudo su www-data
    cd /home/www-data/graphite/store/
    rm -rf whisper
    ln -s /mnt/graphite-data/whisper whisper

If the data disk doesn't exist you will need to create it and attach it using
the following:

  sudo mkdir -p /mnt/graphite-data
  sudo /usr/share/google/safe_format_and_mount -m "mkfs.ext4 -F" \
    /dev/disk/by-id/google-skia-monitoring-data-$ZONE_TAG /mnt/graphite-data

To set the API Key to use for HTML web APIS, use:

    gcutil --project=google.com:skia-buildbots setinstancemetadata skia-monitoring-b --metadata=apikey:[apikey] --fingerprint=[metadata fingerprint]

You can find the current metadata fingerprint by running:

    gcutil --project=google.com:skia-buildbots getinstance skia-monitoring-b

You can find the API Key to use on this page:

    https://console.developers.google.com/project/31977622648/apiui/credential

Do on update
------------

    $ ./vm_push_update.sh

Notes
-----
To SSH into the instance:

    gcutil --project=google.com:skia-buildbots ssh --ssh_user=default skia-monitoring-b

If you need to modify the constants for the vm_XXX.sh scripts they are
specified in compute_engine_scripts/buildbot/vm_config.sh.
