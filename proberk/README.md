# Prober

Prober monitors the uptime of our servers and pumps the results of those probes
into Prometheus.

## Design
Prober periodically polls a set of URLs and uses an appropriate `ResponseTester` function to
make sure the URL is serving the correct response or lack of response.

## Prober Files

Application specific probers should be placed in the applications top level
directory in a file called 'probersk.json'. The `build_probersk_json`
command-line application will incorporate all such files into a single prober
config file that is uploaded to the server. This is run automatically when the container
image is built.

