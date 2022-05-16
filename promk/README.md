# Grafana

The grafana.ini file should almost never change, so if it does,
just delete the pod and have kubernetes restart it so the config
gets read.

Edit the config file by running the ./edit-grafana-config.sh script.

# Prometheus

## Admins

Before deploying yaml files with service accounts you need to give yourself
cluster-admin rights:

      kubectl create clusterrolebinding \
        ${USER}-cluster-admin-binding \
        --clusterrole=cluster-admin \
        --user=${USER}@google.com

## Thanos

The best way to get an idea of all the parts of Thanos and how they work
together is to look at the diagram on the [Thanos
Tuturial](https://thanos.io/quick-tutorial.md/).

There are two protected URLS for Thanos:

- https://thanos-query.skia.org
  - This replaces prom2.skia.org and is allows querying over all metrics over
    all clusters.
- https://thanos-ruler.skia.org
  - This shows all the alert rules being evaluated and their status. You
    probably won't visit this page, as alerts are handled through
    https://am.skia.org.

Both sites above to restricted to Googlers only.

All alert rules are evaluated by [thanos-rule](https://thanos-ruler.skia.org),
which then sends alerts to `alert-to-pubsub`.

**If you add/remove alerts, please run `make update_alerts` to deploy them.**
am.skia.org will take 5-10 minutes to see these changed alerts.

A Thanos sidecar runs alongside each Prometheus instance. For each Prometheus
instance that runs outside of `skia-public` we also run a `thanos-bouncer`
container that sets up a reverse ssh port-forward that allows `thanos-query` to
make queries against the Thanos sidecar.

Additionally `thanos-store` runs in `skia-public` and allows querying against
all the hsitorical data written by the `thanos-sidecar`s.

The long term storage bucket for metrics is `gs://skia-thanos`.

We do not currently run an instance of the Thanos compactor.

## Grafana

Obviously we can't get alerts if `thanos-ruler` stops sending alerts to
`alert-to-pubsub`, so we need a second path for such alerts. We use Grafana's
ability to send alert emails to cover that case. There is a dashboard for Thanos
setup at: https://grafana2.skia.org/d/7giJAG3Wk/thanos?orgId=1 and the Liveness
panel has an alert set if `alert-to-pubsub` goes too long without seeing an
alert come from `thanos-ruler`. When firing the alert will send email to
skiabot@google.com.

# kube-state-metrics

[kube-state-metrics](https://github.com/kubernetes/kube-state-metrics) is run in
all clusters allowing collection of metrics on the state of objects, in
particular states you normally can't get from default metrics, such as cronjobs
that have failed.

To update the version of kube-state-metrics we use the Makefile target
`release_kube-state-metrics` can be updated to use a different tag.