# CockroachDB

Perf supports using CockroachDB as a backend.

There is a single installation of CockroachDB for all the instances of Perf.

There should be a `cockroachdb/connect.sh` script in this directory. Run that
script and you will be dropped into an SQL shell as root to send commands to the
server. You can, for example, create a new database here:

        CREATE DATABASE androidx;

See also: https://www.cockroachlabs.com/docs/stable/authorization.html

You can get to the admin interface for each by running the associated
`cockroachdb/admin.sh` script, which will set up a port-forward to the admin
interface and launch chrome to load the interface.

## Setup

Follow the instructions on [how to set up an instance of CockroachDB on a single
kubernetes cluster](https://www.cockroachlabs.com/docs/stable/orchestrate-cockroachdb-with-kubernetes-insecure.html#manual).

But after downloading the statefulset yaml:

    curl -O https://raw.githubusercontent.com/cockroachdb/cockroach/master/cloud/kubernetes/cockroachdb-statefulset.yaml

The file should be renamed to reflect the Perf instance that will use it, and the
names and app labels in the file should be rename to also reflect the Perf instance, for example:

    s/cockroachdb/perf-cockroachdb/g

The service name and user account are then used in the connection string in the config file, for example:

    {
      "data_store_config": {
      "datastore_type": "cockroachdb",
      "connection_string": "postgresql://root@perf-cockroachdb-public:26257/androidx?sslmode=disable",
      ...
    }

See also [configs](./configs/README.md)

## Migrations

Migrations can be applied from the desktop by using `perf-tool`.

    perf-tool database migrate --config_filename=my-config.json

You might need to forward the CockroachDB connection:

    kubectl port-forward perf-cockroachdb-0 26257

And then override the connection string, for example:

    perf-tool database migrate --config_filename=my-config.json \
      --connection_string=postgresql://root@localhost:26257/mytest?sslmode=disable

## Backups

Migrating between cockroachdb instances can be done by using the 'dump' and
'sql' commands:

    $ cockroach dump --url postgresql://root@perf-flutter-flutter-cockroachdb-public:26257?sslmode=disable flutter  > flutter_flutter.sql

    $ cockroach sql --url postgresql://root@localhost:26257/?sslmode=disable --database flutter_flutter < flutter_flutter.sql

Note that you might want to run this from one of the cockroachdb instances in
the cluster since they have local SSD and the bandwidth is much higher than to
your workstation.
