# Infra Team Launch Checklist

## Assumptions

Your service:

- Is checked in to the `go.skia.org/infra` Git repo.
- Is an HTTP server written in Go with a front end in WebComponents (legacy apps have Polymer)
- Will run in a Docker Container.
- Will use a `*.skia.org` domain.
- Does not handle personal data (additional steps will be required).
- Is not intended to be used by the public at large (additional steps will be required).

[JSFiddle](https://github.com/google/skia-buildbot/tree/master/jsfiddle) is a recent service
that was launched that demonstrates the above. See go/ for the server code and
modules/ and pages/ for the front end.

## Coding

Use `go.skia.org/infra/go/sklog` for logging.

Add flags to your main package like:

    port         = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
    local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
    promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
    resourcesDir = flag.String("resources_dir", "./dist", "The directory to find HTML, JS, and CSS files. If blank the current directory will be used.")


Call `common.InitWithMust([opt], [opt])` in your main function.

Use `go.skia.org/infra/go/login` paired with `infra-sk/modules/login.js`
(Legacy Polymer apps use `res/imp/login.html`) and/or
`go.skia.org/infra/go/webhook` for authentication.

Wrap your `http.Handler` (many services use [mux.NewRouter()](https://github.com/gorilla/mux) with
`go.skia.org/infra/go/httputils.LoggingGzipRequestResponse` to provide monitoring and
logging of HTTP requests and responses. Then, wrap it in
`go.skia.org/infra/go/httputils.HealthzAndHTTPS` to add an unlogged /healthz endpoint for use
with GKE health monitoring and various HTTPS configuration.
Use `go.skia.org/infra/go/httputils.NewTimeoutClient` for HTTP clients, because the default
httpClient doesn't have good defaults for timeouts.

Any calls to external APIs should use an `http.Client` wrapped with
`httputils.AddMetricsToClient`. This allows us to track how much load we place on
external services.

Write your code with security in mind:

- Make sure your service is listed in the [team security scan](http://go/skia-infra-scan).
- Follow the [team security guidelines](http://go/skia-infra-sec) when developing your applicaation.

If you add any critical TODOs while you're coding, file a blocking bug for the issue.


## Makefiles

It is customary to have the following commands in a Makefile for the service.

  - build : Build a development version of the front end.
  - serve : Run the demo pages of the front end in a "watch" mode. This command is for
primary development of front end pages.
  - core : Build the server components.
  - watch : Build a development version of the front end in watch mode. This command is
for running the server, but also making changes to the front end.
  - release : Build a Docker container with the front end and backend parts in it (see below).
  - push : Depends on release, and then pushes to GKE using `pushk`.

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
