SkiaPerf Server
===============

Reads Skia performance data from databases and serves interactive dashboards
for easy exploration and annotations.

Server Setup
============

Please refer to compute_engine_scripts/perfserver/README under the repo for
instructions on creating and destroying the instance. The rest of this document
is what to do once the instance is created.

  gcutil --project=google.com:skia-buildbots addinstance skia-perf-b \
    --zone=us-central2-b --external_ip_address=108.170.220.208 \
    --service_account=default \
    --service_account_scopes="bigquery,storage-full" \
    --network=default --machine_type=n1-standard-1 --image=backports-debian-7-wheezy-v20140605 \
    --persistent_boot_disk

SSH into the instance

  gcutil --project=google.com:skia-buildbots ssh --ssh_user=default skia-perf-b

Do the first time
=================

The following things only need to be done once.

1. SSH into the server as default.
2. sudo apt-get install git
3. git clone https://skia.googlesource.com/buildbot
4. cd ~/buildbot/perf/server/setup
5. ./perf_setup.sh

Change /etc/monit/monitrc to:

    set daemon 2

then run the following so it applies:

    sudo /etc/init.d/monit restart

Then restart squid to pick up the new config file:

    sudo /etc/init.d/squid3 restart

This means that monit will poll every two seconds that our application is up
and running.

To update the code
==================

1. SSH into the server as default.
2. cd ~/buildbot/perf/server/setup
3. git pull
4. ./perf_setup.sh
