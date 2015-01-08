Prober
======

Prober monitors the uptime of our servers and pumps the results of those probes
into InfluxDB.


### Prober ###
The prober requres one piece of project level metadata, the API Key for making
requests to the project hosting API. The API Key value can be found here:

https://console.developers.google.com/project/31977622648/apiui/credential

Set that as the value for the metadata key:

    metadata.APIKEY
