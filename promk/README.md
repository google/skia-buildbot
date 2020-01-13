# Grafana

The grafana.ini file should almost never change, so if it does,
just delete the pod and have kubernetes restart it so the config
gets read.

Edit the config file by running the ./edit-grafana-config.sh script.

TODO(jcgregorio) Backup the sqlite database.

# Prometheus

## Admins

Before deploying yaml files with service accounts you need to give yourself
cluster-admin rights:

      kubectl create clusterrolebinding \
        ${USER}-cluster-admin-binding \
        --clusterrole=cluster-admin \
        --user=${USER}@google.com

## Thanos

All alert rules are evaluated by [thanos-rule](https://thanos-ruler.skia.org),
which then sends alerts to `alert-to-pubsub`.

If an alert is changed only `make push_config_thanos` needs to be run.

A Thanos sidecar runs along side each Prometheus instance. For each Prometheus
instance that runs outside of `skia-public` we also run a `thanos-bouncer`
container that sets up a reverse ssh port-forward that allows `thanos-query` to
make queries against the Thanos sidecar.

Additionally `thanos-store` runs in `skia-public` and allows querying against
all the hsitorical data written by the `thanos-sidecar`s.