# Infra Team Launch Checklist

## Assumptions

Your service:

- is checked in to the `go.skia.org/infra` Git repo
- is an HTTP server written in Go and Polymer
- will run on GCE
- will use a `*.skia.org` domain
- does not handle personal data (additional steps will be required)
- is not intended to be used by the public at large (additional steps will be
  required)

## Coding

Use `github.com/skia-dev/glog` for logging.

Add flags to your main package like:
```
graphiteServer = flag.String("graphite_server", "localhost:2003", "Where is Graphite metrics ingestion server running.")
host           = flag.String("host", "localhost", "HTTP service host")
port           = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
```
You may not need a host flag if your service does not use the login package for
authentication.

Call `common.InitWithMetrics("<service name>", graphiteServer)` in your main
function.

Use `go.skia.org/infra/go/login` paired with `res/imp/9/login.html` and/or
`go.skia.org/infra/go/webhook` for authentication.

Wrap your `http.Handler` with
`go.skia.org/infra/go/util.LoggingGzipRequestResponse` to provide monitoring and
logging of HTTP requests and responses. Use
`go.skia.org/infra/go/util.NewTimeoutClient` for HTTP clients.

Write your code with security in mind. If you add any critical TODOs while
you're coding, file a blocking bug for the issue.

If you need to store database passwords or other data that should not be checked
in to the code, use `go.skia.org/infra/go/metadata` to retrieve the data from
GCE metadata. Add/modify metadata at
[Google Developers Console > Compute > Compute Engine > Metadata](https://pantheon.corp.google.com/project/31977622648/compute/metadata).

## Launching

- Write/update design doc so that others understand how to use, maintain, and
  improve your service.
- Do some back-of-the-envelope calculations to make sure your service can handle
  the expected load. If your service is latency-sensitive, measure the latency
  under load.
- Test on browsers that your users will be using, at least Chrome on desktop and
  ideally Chrome on Android.
- Write VM create and delete scripts, using `compute_engine_scripts/ctfe` as a
  template. Find a free IP address at
  [Google Developers Console > Networking > External IP addresses](https://pantheon.corp.google.com/project/31977622648/addresses/list)
  to use for your instance. Create your instance.
- Write a `build_release` script following the instructions in
  `bash/release.sh`. Write a `.service` file, passing at least these arguments
  to your binary (the `host` flag is not necessary if you do not use the login
  package):
```
--log_dir=/var/log/logserver
--graphite_server=skia-monitoring:2003
--host=<DNS name>.skia.org
```
- Add your server to `push/skiapush.conf` and include `logserverd`, `pulld`, and
  the name given to your package in your `build_release` script. Commit the
  change, build a new `push` release, push `pushd`, run your build_release
  script, and push any out-of-date packages to your instance.
- Add configuration for your service's domain name to
  `skfe/sys/skia_org_nginx`. Commit the change, build a new `skfe` release, and
  push `skfe-config` to `skfe-1` and `-2`. Your service is now live on the
  Internet.
- Add prober rules to `prober/probers.json`.

    - Ideally, probe all public HTML pages and all nullipotent JSON
      endpoints. You can write functions in `prober/go/prober/main.go` to check
      the response body if desired.
    - Probe `<instance name>:10114` to confirm that pulld is running and
      `<instance name>:10115` to confirm the logserverd is running. (Note that
      these are GCE instance names, not DNS names.)

- Add additional stats gathering to your program using
  `github.com/rcrowley/go-metrics`, e.g. to ensure liveness/heartbeat of any
  background processes. You can add stats to see graphs on
  [mon.skia.org](https://mon.skia.org/) even if you do not plan to write alerts
  for these stats.

- Add alert rules to `alertserver/alerts.cfg`. Examples:

    - All prober rules.
    - Additional stats from go-metrics.
    - Too many goroutines.
    - Free disk space on the instance and any attached disks.

- Test prober rules and stats locally using a `prober` running locally and an
  `influxdb` running locally configured as a Graphite server with config like
  `influxdb/influxdb-config.toml`. Build a new `prober` release and push
  `prober`. Build a new `alertserver` release and push `alertserverd`. Check
  that alerts are working correctly.
- Tell people about your new service.
- Be prepared for bug reports. :-)

## Unresolved Questions

Where does this doc belong?

Should there be some security review?

Set up Lemon to fuzz all infra websites on a schedule (maybe weekly)?
https://sites.google.com/a/google.com/lemon-help/
