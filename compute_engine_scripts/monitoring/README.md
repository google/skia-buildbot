Monitoring (Grafana/InfluxDB, Prober/Alert Server)
=====================

The monitoring server runs InfluxDB to accept and manage timeseries data
and uses Grafana to construct dashboards for that data.

In addition this server also hosts the prober, which monitors the uptime
of our servers and pumps the results of those probes into InfluxDB.

Finally the Alert Server (TBD) will periodically query data in InfluxDB
and trigger alerts (emails, pages, sirens, etc) based on the data.

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

### Prober ###
The prober requres one piece of metadata, the API Key for making requests
to the project hosting API. They API Key value can be found here:

https://console.developers.google.com/project/31977622648/apiui/credential

Set that as the value for the metadata key:

    apikey

### Grains ###

Grains is the Grafana/InfluxDB proxy and needs the following metadata values
set:

    cookiesalt
    client_id
    client_secret
    influxdb_name
    influxdb_password

The client_id and client_secret come from here:

    https://console.developers.google.com/project/31977622648/apiui/credential

Look for the Client ID that has a Redirect URI for skiamonitor.com.

For 'cookiesalt' and the influx db values search for 'skiamonitor' in valentine.

Do on update
------------

    $ ./vm_push_update.sh

Notes
-----
To SSH into the instance:

    gcutil --project=google.com:skia-buildbots ssh --ssh_user=default skia-monitoring

If you need to modify the constants for the vm_XXX.sh scripts they are
specified in compute_engine_scripts/buildbots/vm_config.sh.
