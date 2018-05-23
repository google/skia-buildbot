# Infra Team Launch Checklist

## Assumptions

Your service:

- Is checked in to the `go.skia.org/infra` Git repo.
- Is an HTTP server written in Go and Polymer.
- Will run on GCE.
- Will use a `*.skia.org` domain.
- Does not handle personal data (additional steps will be required).
- Is not intended to be used by the public at large (additional steps will be required).

## Coding

Use `go.skia.org/infra/go/sklog` for logging.

Add flags to your main package like:
```
port           = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
local          = flag.Bool("local", false, "Running locally if true. As opposed to in production.")

```

Call `common.InitWithMust([opt], [opt])` in your main function.

Use `go.skia.org/infra/go/login` paired with `res/imp/login.html` and/or
`go.skia.org/infra/go/webhook` for authentication.

Wrap your `http.Handler` with
`go.skia.org/infra/go/httputils.LoggingGzipRequestResponse` to provide monitoring and
logging of HTTP requests and responses. Use
`go.skia.org/infra/go/httputils.NewTimeoutClient` for HTTP clients.

Any calls to external APIs should use an http.Client wrapped with
httputils.AddMetricsToClient. This allows us to track how much load we place on
external services.

Write your code with security in mind:

- Make sure your service is listed in the [team security scan](http://go/skia-infra-scan).
- Follow the [team security guidelines](http://go/skia-infra-sec) when developing your applicaation.

If you add any critical TODOs while you're coding, file a blocking bug for the issue.

## Launching

- Write/update design doc so that others understand how to use, maintain, and
  improve your service.
- Do some back-of-the-envelope calculations to make sure your service can handle
  the expected load. If your service is latency-sensitive, measure the latency
  under load.
- Test on browsers that your users will be using, at least Chrome on desktop and
  ideally Chrome on Android.
- Write VM create and delete program, `vm.go`, using an existing `vm.go` as a
  template. Unless you need a fixed IP address, for MySQL whitelisting for
  example, you should use a dynamic IP address. Find a free IP address at
  [Google Developers Console > Networking > External IP
  addresses](https://console.cloud.google.com/project/31977622648/addresses/list)
  to use for your instance.
- Create your instance.
- Write a `build_release` script following the instructions in
  `bash/release.sh`. Write a `.service` file, passing at least these arguments
  to your binary (the `host` flag is not necessary if you do not use the login
  package):
```
--logtostderr
```
- Add a push description file, `skiapush.json5` in your application directory
  and include `pulld`, and the name given to your package in your
  `build_release` script. Commit the change, build a new `push` release, push
  `pushd`, run your build_release script, and push any out-of-date packages to
  your instance.
- Add metrics endpoints to `prometheus/sys/prometheus.yml` for both the app
  and `pulld` if this is a new server instance. Ensure your job_name matches the
  first argument to `common.InitWithMust`.
- Add configuration for your service's domain name to
  `skfe/sys/skia_org_nginx`. Commit the change, build a new `skfe` release, and
  push `skfe-config` to `skfe-1` and `-2`. Your service is now live on the
  Internet.
- Add prober rules to `probers.json` in your application directory.

    - Ideally, probe all public HTML pages and all nullipotent JSON endpoints.
      You can write functions in `prober/go/prober/main.go` to check the
      response body if desired.

- Add additional stats gathering to your program using
  `go.skia.org/infra/go/metrics2`, e.g. to ensure liveness/heartbeat of any
  background processes. You can add stats to see graphs on
  [mon.skia.org](https://mon.skia.org/) even if you do not plan to write
  alerts for these stats.

- Add alert rules to `prometheus/sys/alert.rules`. The alerts may link
  to a production manual, PROD.md, checked into the application source
  directory. Examples:
    - All prober rules.
    - Additional stats from metrics2.

- Some general metrics apply to all instances and may not need to be added
  explicitly for your application, such as:
    - Too many goroutines.
    - Free disk space on the instance and any attached disks.

- Test prober rules and stats locally using a `prober` running locally and
  checking the probers `/metrics` page. Build a new `prober` release and push
  `prober`. Push prometheus and check that alerts are working correctly.
- Tell people about your new service.
- Be prepared for bug reports. :-)
