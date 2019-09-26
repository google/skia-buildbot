# Production Manual

See [README](./README.md) for the architecture of SKFE.

## To see the currently running envoy pods

    kubectl get pods -lapp=envoy-skia-org

## To see combined logs for all running envoy pods:

    kubectl logs -f -lapp=envoy-skia-org

## Dashboard

[Grafana Dashboard](https://grafana2.skia.org/d/cEEFW_pZk/skfe)

# Alerts

### runtime_load_error

This really shouldn't happen since the entire config for envoy is static and
envoy shouldn't even start with an invalid config. Check envoy logs and maybe
check the filesystem that the config file hasn't become corrupted.

### cluster_bind_error

Check envoy logs for errors. Did a backend go away?

### envoy_cluster_lb_local_cluster_not_ok

Check envoy logs for errors. Did a backend go away?