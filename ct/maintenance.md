# Cluster Telemetry Deployment & Maintenance Guide

The target audience for this document includes Google employees on the Skia
infra team. Other Googlers can request access to the tools and resources
mentioned here by contacting skiabot@. Access is not available to non-Googlers,
but you could create your own setup if desired.

## Intro

General overview of CT is available [here](https://skia.org/dev/testing/ct).

## Code locations

Frontend server code lives in:

- go/ctfe/...
- res/...
- templates/...
- elements.html
- package.json
- sys/...

To build the frontend server, use `make ctfe`. To create a new docker build,
use `make ctfe_release`.
To deploy the latest docker build on the kubernetes ctfe service, use
`make ctfe_push`.

Master code lives in:

- go/master_scripts/...
- go/poller/...
- go/frontend/...
- py/...

To build the master binaries, use `make master_scripts`. To create a new docker
build, use `make ctmaster_release`.
To deploy the latest docker build on the kubernetes ct-master servce, use
`make ctmaster_push`.

Worker code lives in go/worker_scripts/...

To build the worker binaries, use `make worker_scripts`.

Prober config is in probers.json5. Alerts config is in
../promk/prometheus/alerts_public.yml.

## Running locally

### Frontend

To start a local server, run:

```
make ctfe_debug && ctfe --local=true \
  --logtostderr \
  --port=:8000 \
  --host=<your hostname>.cnc.corp.google.com \
  --namespace=cluster-telemetry-staging
```

You can then access the server at [localhost:8000](http://localhost:8000/) or
<your hostname\>.cnc.corp.google.com:8000.

The `host` argument is optional and allows others to log in to your
server. Initially you will get an error when logging in; follow the instructions
on the error page. The `host` argument will also be included in metrics names.

To test prober config changes, edit the config in probers.json5 to
point to localhost:8000, then run `make && prober --alsologtostderr
--use_metadata=false` from ../prober/.

To check metrics from a locally running server or prober, use Prometheus.

### Master

The master poller and master scripts require a /b directory containing various
repos and files. To create and set up this directory, run
`./tools/setup_local.sh` and follow the instructions at the end. (The
`setup_local.sh` script assumes you are a Googler running Goobuntu.)

To run the master poller in dry-run mode (not very useful), run

```
make poller && poller --local=true \
  --alsologtostderr \
  --dry_run \
  --logtostderr
```

To run master scripts locally, you may want to modify the code to skip steps or
exit early, e.g.:
```
diff --git a/ct/go/master_scripts/build_chromium/main.go b/ct/go/master_scripts/build_chromium/main.go
index 1c5c273..34ceb3a 100644
--- a/ct/go/master_scripts/build_chromium/main.go
+++ b/ct/go/master_scripts/build_chromium/main.go
@@ -73,6 +73,11 @@ func main() {
        // Ensure webapp is updated and completion email is sent even if task fails.
        defer updateWebappTask()
        defer sendEmail(emailsArr)
+       if 1 == 1 {
+               time.Sleep(10 * time.Second)
+               taskCompletedSuccessfully = true
+               return
+       }
        // Cleanup tmp files after the run.
        defer util.CleanTmpDir()
        // Finish with glog flush and how long the task took.
```
- Master scripts include `build_chromium`, `capture_archives_on_workers`,
  `capture_skps_on_workers`, `create_pagesets_on_workers`,
  `run_chromium_perf_on_workers`.

You can run the poller as:

```
make poller && poller --local=true \
  --alsologtostderr \
  --logtostderr
```

### Workers

The Makefile has examples of running the worker scripts locally.

TODO(benjaminwagner): Add local flag and kill e2e_tests from Makefile.

## Running in prod

### Frontend

The CTFE production datastore instance is
[here](https://console.cloud.google.com/datastore/entities/query?organizationId=433637338589&project=google.com:skia-buildbots&ns=cluster-telemetry&kind=CaptureSkpsTasks).
The staging datastore instance is
[here](https://console.cloud.google.com/datastore/entities/query?organizationId=433637338589&project=google.com:skia-buildbots&ns=cluster-telemetry-staging&kind=CaptureSkpsTasks).

The frontend runs on a Google Cloud Kubernetes service named
[ctfe](https://console.cloud.google.com/kubernetes/service/us-central1-a/skia-public/default/ctfe?project=skia-public&organizationId=433637338589).
Its dockerfile is in ctfe/Dockerfile.

To build the frontend server, use `make ctfe`. To create a new docker build,
use `make ctfe_release`.
To deploy the latest docker build on the kubernetes ctfe service, use
`make ctfe_push`. You can then see the updated frontend at
[ct.skia.org](https://ct.skia.org/).

To access ctfe directly, use `kubectl exec -it $(kubectl get pod
--selector="app=ctfe" -o jsonpath='{.items[0].metadata.name}') bash`.

Frontend logs are available [here](https://console.cloud.google.com/logs/viewer?project=skia-public&advancedFilter=logName%3D%22projects%2Fskia-public%2Flogs%2Fctfe%22).

### Master

The poller and master scripts run on a Google cloud kubernetes service named
[ct-master](https://console.cloud.google.com/kubernetes/service/us-central1-a/skia-public/default/ct-master?project=skia-public&organizationId=433637338589).
Its dockerfile is in ct-master/Dockerfile.

To push a new build to the poller safely, check that the CTFE task queue is
empty and then run `make ctmaster_release && make ctmaster_push`.

To access ct-master directly, use `kubectl exec -it $(kubectl get pod
--selector="app=ct-master" -o jsonpath='{.items[0].metadata.name}') bash`.

Poller logs are available [here](https://console.cloud.google.com/logs/viewer?project=skia-public&advancedFilter=logName%3D%22projects%2Fskia-public%2Flogs%2Fct-master%22).

### Workers

Worker scripts are part of the docker build when `ctmaster_release` is run.
However, from time to time, it may be necessary to perform maintenance
tasks on all worker machines. In this case, the
[run_on_swarming_bots](https://skia.googlesource.com/buildbot/+show/master/scripts/run_on_swarming_bots/)
script can be used to update all
[CT bots](https://chrome-swarming.appspot.com/botlist?c=id&c=os&c=task&c=status&f=pool%3ACT&l=1000&s=id%3Aasc).

## Other maintenance

### Updating pagesets

TODO(rmistry): Where do CSV files come from, where to put in GS.

## Access to Golo

CT's Golo bots are visible [here](https://chrome-swarming.appspot.com/botlist?c=id&c=task&c=os&c=status&d=asc&f=pool%3ACT&k=zone&s=id).

To log in to Golo bots, see [go/chrome-infra-build-access](http://go/chrome-infra-build-access).
