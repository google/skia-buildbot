# Prometheus Pushgateway

Sometimes we need to monitor components which cannot be scraped. Examples: CLI
processes and task drivers.

Prometheus [recommends](https://prometheus.io/docs/instrumenting/pushing/) using
the [Prometheus Pushgateway](https://github.com/prometheus/pushgateway) for
pushing time series from short-lived service-level batch jobs.

The Prometheus Pushgateway exists to allow ephemeral and batch jobs to expose
their metrics to Prometheus. Since these kinds of jobs may not exist long enough
to be scraped, they can instead push their metrics to a Pushgateway. The
Pushgateway then exposes these metrics to Prometheus.


## k8s service

Skia runs a [pushgateway service](https://skia.googlesource.com/k8s-config/+/main/skia-public/pushgateway.yaml)
in k8s. The service runs with an auth-proxy container to provide authentication
to the pushgateway container ports. The pushgateway container also specifies a
[persistence.file](https://github.com/prometheus/pushgateway#run-it) to persist
metrics across container restarts.

The service is hosted at pushgateway.skia.org


## pushgateway.go

Contains convenience methods for interacting with Skia's pushgateway service.
