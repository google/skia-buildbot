Grafana
=======

The grafana.ini file should almost never change, so if it does,
just delete the pod and have kubernetes restart it so the config
gets read.

Edit the config file by running the ./edit-grafana-config.sh script.

TODO(jcgregorio) Backup the sqlite database.

Prometheus
==========

Admins
------

Before deploying yaml files with service accounts you need to give yourself
cluster-admin rights:

      kubectl create clusterrolebinding \
        ${USER}-cluster-admin-binding \
        --clusterrole=cluster-admin \
        --user=${USER}@google.com

