# Grafana

The grafana.ini file should almost never change, so if it does,
just delete the pod and have kubernetes restart it so the config
gets read.

Edit the config file by running the ./edit-grafana-config.sh script.

# Prometheus

See http://go/skia-infra-metrics

# kube-state-metrics

[kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) is run in
all clusters allowing collection of metrics on the state of objects, in
particular states you normally can't get from default metrics, such as cronjobs
that have failed.

To update the version of kube-state-metrics find a later build in the [official
builds](https://pantheon.corp.google.com/gcr/images/google-containers/global/kube-state-metrics)
and update all the YAML files in k8s-config.