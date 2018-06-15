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

To build the frontend server, use `make ctfe`. To create a new DEB for
[push](https://push.skia.org/), use `MESSAGE="<release message>" make
ctfe_release`.

Master code lives in:

- go/master_scripts/...
- go/poller/...
- go/frontend/...
- py/...

To build the master binaries, use `make master_scripts`. The poller
automatically runs `git pull` and `make all` to update the master scripts before
running any tasks, so changes will be live as soon as they are committed; the
next task to start will run the updated code. The poller itself must be updated
manually; see below.

Worker code lives in go/worker_scripts/...

To build the worker binaries, use `make worker_scripts`. Master scripts
run `git pull` and `make all` on all slaves before starting the worker scripts.

Prober config is in ../prober/probers.json. Alerts config is in
../alertserver/alerts.cfg.

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

To test prober config changes, edit the config in ../prober/probers.json to
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
  `run_chromium_perf_on_workers`, and `run_lua_on_workers`.

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

The frontend runs on a GCE instance named
[skia-ctfe](https://console.cloud.google.com/project/31977622648/compute/instancesDetail/zones/us-central1-c/instances/skia-ctfe).
There are scripts to create and delete this instance in
../compute_engine_scripts/ctfe/.

As mentioned above, to create a new DEB for CTFE, use `MESSAGE="<release
message>" make ctfe_release`. To deploy this DEB to the skia-ctfe instance, use
[push](https://push.skia.org/). Systemd will automatically restart the service
after the update is installed, based on the configuration in
sys/ctfe.service. You can then see the updated frontend at
[ct.skia.org](https://ct.skia.org/).

To access skia-ctfe directly, use `gcloud compute ssh default@skia-ctfe --zone
us-central1-c`.

Frontend logs are available via the
link from [push](https://push.skia.org/).

### Master

The poller and master scripts run on skia-ct-master in the skia-buildbots
Google cloud project. See
[here](https://skia.googlesource.com/buildbot/+/master/compute_engine_scripts/ct_master/)
for how the GCE instance is setup and
[here](https://skia.googlesource.com/buildbot/+/master/ct/sys/ct-masterd.service)
for how the poller is started.

To stop the poller safely, check that the CTFE task queue is empty and then
stop ct-masterd in skia-ct-master on push.

Changes to master scripts and worker scripts will be picked
up for the next task execution after the change is committed.

### Workers

Worker scripts are normally automatically updated and run via the master
scripts. However, from time to time, it may be necessary to perform maintenance
tasks on all worker machines. In this case, the
[run_on_swarming_bots](https://skia.googlesource.com/buildbot/+/master/scripts/run_on_swarming_bots/)
script can be used to update all
[CT bots](https://chrome-swarming.appspot.com/botlist?c=id&c=os&c=task&c=status&f=pool%3ACT&l=1000&s=id%3Aasc).

## Other maintenance

### Updating pagesets

TODO(rmistry): Where do CSV files come from, where to put in GS.

## Access to Golo

Follow instructions
[here](https://chrome-internal.googlesource.com/infra/infra_internal/+/master/doc/ssh.md)
for basic security key and `.ssh/config` setup.

Run `ssh -p 2150 skia-telemetry-ssh@chromegw` and use the password stored on
[Valentine](https://valentine.corp.google.com/) as "Chrome Labs (b5) -
skia-telemetry-ssh". Then follow
[these instructions](https://g3doc.corp.google.com/ops/cisre/corpssh/g3doc/faq/index.md?cl=head#pubkey_external)
to add your gnubby public key to `~/.ssh/authorized_keys2` on vm0-m5 (please
take care to append rather than overwrite).

Add the following to your `.ssh/config`:
```
Host *5.golo
  ProxyCommand ssh -p 2150 -oPasswordAuthentication=no skia-telemetry-ssh@chromegw nc %h.chromium.org %p
```

Run `ssh build101-m5.golo` and use the password stored on
[Valentine](https://valentine.corp.google.com/) as
"skia-telemetry-chrome-bot". Add your gnubby public key to
`~/.ssh/authorized_keys2` on build101-m5 as well.

You can now use `ssh build101-m5.golo` and tap your security key twice to log in
without a password (although you will need enter your security key password once
per powercycle). This setup does not require prodaccess.

