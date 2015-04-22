Grains
======

Grains is the Grafana/InfluxDB proxy server.


### Grains ###
It needs the following project level metadata set:

    metadata.COOKIESALT
    metadata.CLIENT_ID
    metadata.CLIENT_SECRET
    metadata.INFLUXDB_NAME
    metadata.INFLUXDB_PASSWORD

The client_id and client_secret come from here:

    https://console.developers.google.com/project/31977622648/apiui/credential

Look for the Client ID that has a Redirect URI for mon.skia.org.

For 'cookiesalt' and the influx db values search for 'skiamonitor' in valentine.
