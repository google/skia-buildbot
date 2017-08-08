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

Prober Files
------------

Application specific probers should be placed in the applications top level
directory in a file called 'probers.json5'. The `build_probers_json5`
command-line application will incorporate all such files into a single prober
config file that is uploaded to the server.
