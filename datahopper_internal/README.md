datahopper_internal
-------------------

Pulls data from Google-internal sources, such as the Android Build APIs and
Google3 builds, and funnels that into the buildbot database.

Requires the following bits of project level Metadata:

  * datahopper_internal_targets - A space separated list of tradefed targets.
  * cookieSalt - Salt for cookie generation.
  * jwt_service_account - JWT JSON Service Account data.
  * database_readwrite - Password for the build database read/write access.
  * webhook_request_salt - Salt for authenticating webhook requests.

The instance must also be setup with the scope
"https://www.googleapis.com/auth/androidbuild.internal" for the compute engine
service account.

### Running locally

You will also need a file named "service-account.json" in the CWD containing the
value of the
[GCE metadata key jwt_service_account](https://console.cloud.google.com/project/31977622648/compute/metadata).

To start a local server, start up a local task_scheduler, then run:

```
mkdir /tmp/datahopper_internal
make && datahopper_internal --local=true \
  --logtostderr \
  --port=:8000 \
  --targets=<ask another infra team member> \
  --workdir=/tmp/datahopper_internal \
  --codename_db_dir=/tmp/datahopper_internal_codenames \
  --task_scheduler_url=https://localhost:8001/json/task \
  --task_db_url=https://localhost:8008/db/
```

To test the ingestBuild handler locally, use commands like the following:

```
DATA='{"target": "Google3-Autoroller",
    "commitHash": "0c0da2b0e21e37c33ae577e0af3e921df320ce5b",
    "status": "success",
    "changeListNumber": "111378794",
    "startTime": "1485453372",
    "finishTime": "1485453455"}'
AUTH="$(echo -n "${DATA}notverysecret" | sha512sum | xxd -r -p - | base64 -w 0)"
curl -v -H "X-Webhook-Auth-Hash: $AUTH" -d "$DATA" http://localhost:8000/ingestBuild
```
