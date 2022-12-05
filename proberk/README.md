# Prober

Prober monitors the uptime of our servers and pumps the results of those probes
into Prometheus.

## Design
Prober periodically polls a set of URLs and uses an appropriate `ResponseTester` function to
make sure the URL is serving the correct response or lack of response.

## Prober Files

Application specific probers should be placed in the //prober directory of the
k8s-config repo in a file named for the application. The `generate.sh` script
will incorporate all such files into a single prober config file that is
included in the proberk Docker image.
