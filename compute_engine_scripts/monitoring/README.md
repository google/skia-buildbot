Monitoring (Grains, Prober and Alert Server)
============================================

The monitoring server runs InfluxDB to accept and manage timeseries data and
uses Grafana to construct dashboards for that data. InfluxDB has a module to
make it compatible with Graphite/Carbon, which we used to use to store
timeseries data before InfluxDB. Our servers still upload metrics using this
Graphite/Carbon API, so you'll see mentions of Graphite or Carbon here and
there.

Logs for all applications are served from skiamonitor.com:10115 which is
restricted to internal IPs only.

Full Server Setup
=================

Do once
-------

    $ ./vm_create_instance.sh
    $ ./vm_setup_instance.sh

Make sure to 'set daemon 2' in /etc/monit/monitrc so that monit
runs every 2 seconds.

Make sure to log in InfluxDB at port 10117 and create the 'graphite' and
'grafana' databases. Username and Password should also be set according to
valentine.

Once that is done then set the Metadata for the instance using
cloud.google.com/console, see below:

Do on update
------------

    $ ./vm_push_update.sh

Notes
-----
To SSH into the instance:

    gcutil --project=google.com:skia-buildbots ssh --ssh_user=default skia-monitoring

If you need to modify the constants for the vm_XXX.sh scripts they are
specified in vm_config.sh.
