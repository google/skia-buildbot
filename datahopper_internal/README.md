datahopper_internal
-------------------

Pulls data from Google-internal sources, such as the Android Build APIs and
Google3 builds, and funnels that into the buildbot database.

Requires the following bits of project level Metadata:

  * datahopper_internal_targets - A space separated list of tradefed targets.
  * cookieSalt - Salt for cookie generation.
  * jwt_service_account - JWT JSON Service Account data.
  * database_readwrite - Password for the build database read/write access.

The instance must also be setup with the scope
"https://www.googleapis.com/auth/androidbuild.internal" for the compute engine
service account.

### Running locally

You will need to install InfluxDB locally and configure it as a graphite
server using the configuration in ../influxdb/influxdb-config.toml.

You will also need a file named "service-account.json" in the CWD containing the
value of the
[GCE metadata key jwt_service_account](https://pantheon.corp.google.com/project/31977622648/compute/metadata).

To start a local server, `mkdir /tmp/datahopper_internal`, then run:

```
make && datahopper_internal --local=true \
  --logtostderr \
  --port=:8000 \
  --graphite_server='localhost:2003' \
  --targets=<ask another infra team member> \
  --workdir=/tmp/datahopper_internal \
  --codename_db_dir=/tmp/datahopper_internal_codenames
```

To test the ingestBuild handler locally, use commands like the following:

```
DATA='{"target": "Google3-Autoroller",
    "commitHash": "279c7864090a7b96c34c3594e38ced35967c673f",
    "status": "failure",
    "changeListNumber": "111378794"}'
AUTH="$(echo -n "${DATA}notverysecret" | sha512sum | xxd -r -p - | base64 -w 0)"
curl -v -H "X-Webhook-Auth-Hash: $AUTH" -d "$DATA" http://localhost:8000/ingestBuild
```
