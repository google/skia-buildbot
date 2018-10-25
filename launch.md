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

  - `build` : Build a development version of the front end.
  - `serve` : Run the demo pages of the front end in a "watch" mode. This command is for
primary development of front end pages.
  - `core` : Build the server components.
  - `watch` : Build a development version of the front end in watch mode. This command is
for running the server, but also making changes to the front end.
  - `release` : Build a Docker container with the front end and backend parts in it (see below).
  - `push` : Depends on release, and then pushes to GKE using `pushk`.

## Docker

Running apps in Docker makes deployment and local testing much much easier.
It additionally allows integration with GKE. Some legacy apps are not yet
run in Docker, but it is the goal to have everything on GKE+Docker.

Create a Dockerfile for your app in the root of the project folder
(e.g. `jsfiddle/Dockerfile`). If there are multiple services, put them in a named folder
(e.g. `fiddlek/fiddle/Dockerfile`, `fiddlek/fiddler/Dockerfile`).

When choosing a base image, consider our light wrappers, found in `kube/*`.
For example, `kube/basealpine/Dockerfile` which can be used by having
`FROM gcr.io/skia-public/basealpine:3.7` as the first line in a Dockerfile.

We have a helper script for 'installing' an app into a Docker container,
`bash/docker_build.sh`. A call to this script is customarily put in a bash
script which is called by `make release`. See `jsfiddle/build_release`
for an example. To integrate docker_build.sh into the actual container, add
a `COPY . /` to copy the executable(s) and HTML/JS/CSS from the build context
into the container. Legacy apps have a similar set-up, but for building a
Debian package instead of a container.

It is customary to include an ENTRYPOINT and CMD with sensible defaults
for the app. It's also a best practice to run the app as USER skia unless
root is absolutely needed.

Putting all the above together, a bare-bones Dockerfile would look something like:

    FROM gcr.io/skia-public/basealpine:3.7

    COPY . /

    USER skia

    ENTRYPOINT ["/usr/local/bin/my_app_name"]
    CMD ["--logtostderr", "--port=:8000", "--resources_dir=/usr/local/share/my_app_name/"]

## Secrets and Service Accounts

If your app needs access to a GCS bucket or other similar things, it is recommended
you create a new service account for your app. See below for linking it into the
container.

Use an existing `create-sa.sh` script (e.g. `create-jsfiddle-sa.sh`) and tweak
the name, committing it into the app's root directory. Run this once to create
the service account and create the secrets in GKE.

## Launching

- Write/update design doc so that others understand how to use, maintain, and
  improve your service. `DESIGN.md` typically has high level design structures
  (e.g. where is data stored, how do the pieces of software interact, etc).
  `PROD.md` has an overview of the alerts and any other notes for maintaining the
  service.
- Do some back-of-the-envelope calculations to make sure your service can handle
  the expected load. If your service is latency-sensitive, measure the latency
  under load.
- Test on browsers that your users will be using, at least Chrome on desktop and
  ideally Chrome on Android.
- Create an app.yaml in [skia-public-config](https://skia.googlesource.com/skia-public-config/+/master/)
This controls how your app will be run in GKE. See
[these docs](https://kubernetes.io/docs/concepts/services-networking/connect-applications-service/)
for more on the schema. Commit this, then run `pushk appname` to make the configuration
active.
- Metrics are customarily made available at port 20000. To configure metrics scraping,
  add the following to the app.yaml under spec -> template -> metadata:

```yaml
annotations:
  prometheus.io.scrape: "true"
  prometheus.io.port: "20000"
```

- Metrics will be available on [prom2.skia.org](https://prom2.skia.org/). Legacy apps report metrics
to [prom.skia.org](https://prom.skia.org/) and require updating `prometheus/sys/prometheus.yml`.
- The metrics will be labeled `app=<foo>` where `foo` is the first argument to
  `common.InitWithMust`.
- If you have secrets (like a service account), bind it to the deployment by adding
the following to app.yaml:

```yaml
spec:
  automountServiceAccountToken: false
  ...
  containers:
    - name: my-container
      ...
      volumeMounts:
        - name: my-app-sa
          mountPath: /var/secrets/google
      env:
        - name: GOOGLE_APPLICATION_CREDENTIALS
          value: /var/secrets/google/key.json
      ...
  volumes:
    - name: my-app-sa
      secret:
        secretName: my-app
```

- Update [skia-ingress.yaml](https://skia.googlesource.com/skia-public-config/+/master/skia-ingress.yaml)
with an entry for your app. Note that this is in the `skia-public-config` repo.
Commit this, then run `kubectl apply -f skia-ingress.yaml` to apply the new config.
- Add configuration for your service's domain name to
  `skfe/sys/skia_org_nginx`.
  - You basically want to do a proxy_pass to the skia-public's IP address. This will connect to
    the skia-ingress config.
  - Legacy apps link to their respective VM.
  - Commit the change, build a new `skfe` release, and
  push `skfe-config` to `skfe-1` and `-2`. Your service is now live on the
  Internet.

- Run `./kube/set-backend-timeouts.sh` to fix the default HTTP timeout length.
- Add prober rules to `probers.json` in your application directory.

    - Ideally, probe all public HTML pages and all nullipotent JSON endpoints.
      You can write functions in `prober/go/prober/main.go` to check the
      response body if desired.

- Add additional stats gathering to your program using
  `go.skia.org/infra/go/metrics2`, e.g. to ensure liveness/heartbeat of any
  background processes.

- Add alert rules to [alerts_public](https://skia.googlesource.com/buildbot/+/master/promk/prometheus/alerts_public.yml).
   The alerts may link to a production manual, `PROD.md`, checked into the
  application source directory. Examples:
    - All prober rules.
    - Additional stats from metrics2.
  Legacy apps have their alert rules in `prometheus/sys/alert.rules`

- Some [general metrics](https://skia.googlesource.com/buildbot/+/master/promk/prometheus/alerts_general.yml)
apply to all apps and may not need to be added
  explicitly for your application, such as:
    - Too many goroutines.
    - Free disk space on the instance and any attached disks.
- Tell people about your new service.
- Be prepared for bug reports. :-)
