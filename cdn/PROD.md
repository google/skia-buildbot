# CDN Production Manual

General information about the service is available in the [README](./README.md).

# Alerts

## error_rate

An elevated error rate likely indicates some problem communicating with GCS,
which may point to a configuration problem or some transient problem with GCS
itself. The CDN service may also log errors for bad requests, eg. when an object
is requested which does not exist. This may be caused by an individual user
making a mistake or by misconfiguration of another service. Check the
[logs](https://pantheon.corp.google.com/logs/query;query=resource.labels.container_name%3D%22cdn%22?project=skia-public)
and determine the root cause.
