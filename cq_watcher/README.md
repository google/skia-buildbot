CQ Watcher
==========

The CQ Watcher monitors the Skia CQ and how long trybots in the CQ take. It
pumps the results of the monitoring into InfluxDB.


TODO(rmistry): Below needed??
### Prober ###
The prober requres one piece of project level metadata, the API Key for making
requests to the project hosting API. The API Key value can be found here:

https://console.developers.google.com/project/31977622648/apiui/credential

Set that as the value for the metadata key:

    metadata.APIKEY
