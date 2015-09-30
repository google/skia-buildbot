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
- go/ctfe_migratedb/...
- res/...
- templates/...
- elements.html
- package.json
- sys/...

To build the frontend server, use `make ctfe`. To create a new DEB for
[push](https://push.skia.org/), use `MESSAGE="<release message>" make
ctfe_release`.

The frontend DB schema is defined in go/db/db.go.

Master code lives in:

- go/master_scripts/...
- go/poller/...
- go/frontend/...
- py/...

To build the master binaries, use `make master_scripts`. The poller
automatically runs `go get -u ./...` and `make all` to update the master scripts
before running any tasks, so changes will be live as soon as they are committed;
the next task to start will run the updated code. The poller itself must be
updated manually; see below.

Worker code lives in go/worker_scripts/...

To build the worker binaries, use `make worker_scripts`. Master scripts
run `go get -u ./...` and `make all` on all slaves before starting the worker
scripts.

Prober config is in ../prober/probers.json. Alerts config is in
../alertserver/alerts.cfg.

## Running locally

### Frontend

To set up or upgrade the CTFE DB, run `make ctfe && ctfe_migratedb
--ctfe_db_host=localhost --logtostderr --ctfe_db_user=root`. Occasionally you
may find it useful to downgrade the local DB; specify the `--target_version`
flag to achieve this. Run `mysql -u root -D ctfe` to access the DB using SQL.

To start a local server, run:
```
make ctfe_debug && ctfe --local=true \
  --logtostderr \
  --ctfe_db_host=localhost \
  --port=:8000 \
  --ctfe_db_user=readwrite \
  --graphite_server='localhost:2003'
  --host=<your hostname>.cnc.corp.google.com
```
You can then access the server at [localhost:8000](http://localhost:8000/) or
<your hostname\>.cnc.corp.google.com:8000.

The `graphite_server` argument is only needed when testing metrics gathering;
you will need to install InfluxDB locally and configure it as a graphite
server using the configuration in ../influxdb/influxdb-config.toml.

The `host` argument is optional and allows others to log in to your
server. Initially you will get an error when logging in; follow the instructions
on the error page. The `host` argument will also be included in metrics names.

To test prober config changes, edit the config in ../prober/probers.json to
point to localhost:8000, then run `make && prober --alsologtostderr
--use_metadata=false` from ../prober/.

To check metrics from a locally running server or prober, run:
```
influx --port 10117
> use graphite
> show measurements
> select * from /ctfe.benjaminwagner1-cnc-corp-google-com.num-pending-tasks.value/;
```
replacing the value within slashes with whichever measurement you have changed.

To test alert config changes:

1. Run `make migratedb && alertserver_migratedb --db_host=localhost
   --logtostderr` from ../alertserver/.
2. Edit the config in ../alertserver/alerts.cfg to make the measurement names
   match what you see in InfluxDB (you may need to comment out measurements that
   don't exist in your local InfluxDB).
3. TODO(benjaminwagner): Run `make all && alertserver --alsologtostderr
   --testing --use_metadata=false --alert_db_host=localhost
   --buildbot_db_host=localhost --buildbot_db_user=root --influxdb_host
   localhost:10117 --influxdb_database=graphite`

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
  --log_dir=/b/storage/glog \
  --graphite_server='localhost:2003'
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
  --log_dir=/b/storage/glog \
  --graphite_server='localhost:2003'
```

### Workers

The Makefile has examples of running the worker scripts locally.

TODO(benjaminwagner): Add local flag and kill e2e_tests from Makefile.

## Running in prod

### Frontend

The CTFE DB is a
[Cloud SQL](https://pantheon.corp.google.com/project/31977622648/sql/instances)
instance named ctfe. The IP address of this instance is the default value of the
`--ctfe_db_host` argument. The DB is created/upgraded automatically with an
`ExecStartPre` configuration in sys/ctfe.service.

The frontend runs on a GCE instance named
[skia-ctfe](https://pantheon.corp.google.com/project/31977622648/compute/instancesDetail/zones/us-central1-c/instances/skia-ctfe).
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

Frontend logs are available [here](http://104.154.112.110:10115/) or via the
link from [push](https://push.skia.org/).

### Master

The poller and master scripts run on build101-m5 in the Chrome Golo. See below
for how to access the Golo.

There is a script named `setup_cluster_telemetry_machine.sh` on build101-m5 (and
checked into git in the tools directory) that will set up the machine for
running the poller and master scripts. It's unlikely that you will need to run
this script now that the machine is set up, and the script is currently
out-of-date. There are a few files that the script does not create
automatically:

- /b/storage/webhook_salt.data: Set this file to the value of
  [GCE metadata key webhook_request_salt](https://pantheon.corp.google.com/project/31977622648/compute/metadata).
- /b/storage/email.data and /b/storage/google_storage_token.data: Sign in to
  `skia.buildbots@gmail.com` in your browser using the password stored in
  [Valentine](https://valentine.corp.google.com/) as
  "skia.buildbots@gmail.com". Then run one of the master scripts,
  e.g. `build_chromium`. Follow the prompts in the output, once to authenticate
  for email and once to authenticate to Google Storage.

There are several aliases in .bashrc on build101-m5 that are useful for
maintenance. If the poller is not running (check using `ps x | grep poller`),
run `start_poller` to update the code from the infra Git repo and start the
poller. The definition of `start_poller` is currently:
```
start_poller () {
  cd /b/skia-repo/go/src/go.skia.org/infra/ct/
  git pull
  make all
  nohup poller --log_dir=/b/storage/glog --graphite_server=<IP address>:2003 &
}
```
where `<IP address>` is the external IP address for the skia-monitoring
instance, available
[here](https://pantheon.corp.google.com/project/31977622648/compute/instancesDetail/zones/us-central1-c/instances/skia-monitoring?graph=GCE_CPU&duration=PT1H).

To stop the poller safely, check that the CTFE task queue is empty and check
with `pstree <PID of poller>` to verify that there are no master scripts or
check_workers_health currently running, then kill the poller process.

As mentioned above, changes to master scripts and worker scripts will be picked
up for the next task execution after the change is committed.

Master logs are available
[here](https://uberchromegw.corp.google.com/i/skia-ct-master/).

### Workers

Worker scripts are normally automatically updated and run via the master
scripts. However, from time to time, it may be necessary to perform maintenance
tasks on all worker machines. In this case, the `run_command` master script can
be used to run a command on all worker machines:
```
ssh build101-m5.golo
run_command --cmd 'echo "Hello World!"' --logtostderr=true --timeout=60m
```

You will likely want to follow the procedure above to stop the poller before
performing maintenance on the workers.

### Logserver

In order to serve
[the master and worker logs](https://uberchromegw.corp.google.com/i/skia-ct-master/all_logs),
each CT machine runs logserver and logserver_proxy. There are entries in the
crontab for each machine to start logserver and logserver_proxy every 5 minutes
(no-op if already running). The code for logserver is under ../logserver and the
code for logserver_proxy is under go/master_scripts/logserver_proxy.

You can update logserver by doing something like:
```
ssh build101-m5.golo
cd /b/skia-repo/go/src/go.skia.org/infra/logserver; go get -u ./...; make; killall logserver
run_command --cmd 'cd /b/skia-repo/go/src/go.skia.org/infra/logserver; go get -u ./...; make; killall logserver' --logtostderr=true --timeout=60m
```

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

