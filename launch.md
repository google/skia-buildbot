# Infra Team Launch Checklist

## Assumptions

Your service:

- Is checked in to the `go.skia.org/infra` Git repo.
- Is an HTTP server written in Go with a front end in WebComponents (legacy apps
  have Polymer)
- Will run in a Docker Container.
- Will use a `*.skia.org` domain.
- Does not handle personal data (additional steps will be required).
- Is not intended to be used by the public at large (additional steps will be
  required).

[JSFiddle](https://github.com/google/skia-buildbot/tree/master/jsfiddle) is a
recent service that was launched that demonstrates the above. See go/ for the
server code and modules/ and pages/ for the front end.

## Coding

Use `go.skia.org/infra/go/sklog` for logging.

Add flags to your main package like:

    port         = flag.String("port", ":8002", "HTTP service port (e.g., ':8002')")
    local        = flag.Bool("local", false, "Running locally if true. As opposed to in production.")
    promPort     = flag.String("prom_port", ":20000", "Metrics service address (e.g., ':10110')")
    resourcesDir = flag.String("resources_dir", "./dist", "The directory to find HTML, JS, and CSS files. If blank the current directory will be used.")

Call `common.InitWithMust([opt], [opt])` in your main function.

Use `go.skia.org/infra/go/login` paired with
`../../../infra-sk/modules/login.ts` (Legacy Polymer apps use
`res/imp/login.html`) and/or `go.skia.org/infra/go/webhook` for authentication.
When using OAuth, see the secrets section below for including client secrets.

Wrap your `http.Handler` (many services use
[mux.NewRouter()](https://github.com/gorilla/mux) with
`go.skia.org/infra/go/httputils.LoggingGzipRequestResponse` to provide
monitoring and logging of HTTP requests and responses. Then, wrap it in
`go.skia.org/infra/go/httputils.HealthzAndHTTPS` to add an unlogged /healthz
endpoint for use with GKE health monitoring and various HTTPS configuration.

Use `go.skia.org/infra/go/httputils.DefaultClientConfig` for HTTP clients, which
provides several features:

- ensures requests time out within a reasonable limit
- tracks how much load we place on external services
- optionally adds authentication to requests
- optionally adds automatic retries with exponential backoff
- optionally treats non-2xx responses as errors

Write your code with security in mind:

- Make sure your service is listed in the
  [team security scan](http://go/skia-infra-scan).
- Follow the [team security guidelines](http://go/skia-infra-sec) when
  developing your applicaation.

If you add any critical TODOs while you're coding, file a blocking bug for the
issue.

If your application requires Puppeteer tests, it should be explicitly opted in
by making any necessary changes to //puppeteer-tests/docker/run-tests.sh.

## Makefiles

It is customary to have the following commands in a Makefile for the service.

- `build` : Build a development version of the front end.
- `serve` : Run the demo pages of the front end in a "watch" mode. This command
  is for primary development of front end pages.
- `core` : Build the server components.
- `watch` : Build a development version of the front end in watch mode. This
  command is for running the server, but also making changes to the front end.
- `release` : Build a Docker container with the front end and backend parts in
  it (see below).
- `push` : Depends on release, and then pushes to GKE using `pushk`.

## Docker

Running apps in Docker makes deployment and local testing much much easier. It
additionally allows integration with GKE. Some legacy apps are not yet run in
Docker, but it is the goal to have everything on GKE+Docker.

Create a Dockerfile for your app in the root of the project folder (e.g.
`jsfiddle/Dockerfile`). If there are multiple services, put them in a named
folder (e.g. `fiddlek/fiddle/Dockerfile`, `fiddlek/fiddler/Dockerfile`).

When choosing a base image, consider our light wrappers, found in `kube/*`. For
example, `kube/basealpine/Dockerfile` which can be used by having
`FROM gcr.io/skia-public/basealpine:3.9` as the first line in a Dockerfile.

We have a helper script for 'installing' an app into a Docker container,
`bash/docker_build.sh`. A call to this script is customarily put in a bash
script which is called by `make release`. See `jsfiddle/build_release` for an
example. To integrate docker_build.sh into the actual container, add a
`COPY . /` to copy the executable(s) and HTML/JS/CSS from the build context into
the container. Legacy apps have a similar set-up, but for building a Debian
package instead of a container.

It is customary to include an ENTRYPOINT and CMD with sensible defaults for the
app. It's also a best practice to run the app as USER skia unless root is
absolutely needed.

Putting all the above together, a bare-bones Dockerfile would look something
like:

    FROM gcr.io/skia-public/basealpine:3.9

    COPY . /

    USER skia

    ENTRYPOINT ["/usr/local/bin/my_app_name"]
    CMD ["--port=:8000", "--resources_dir=/usr/local/share/my_app_name/"]

## Secrets and Service Accounts

If your app needs access to a GCS bucket or other similar things, it is
recommended you create a new service account for your app. See below for linking
it into the container.

Use an existing `create-sa.sh` script (e.g. `create-jsfiddle-sa.sh`) and tweak
the name, committing it into the app's root directory. Run this once to create
the service account and create the secrets in GKE.

## Authentication

Almost all applications should use
[google.DefaultTokenSource()](https://pkg.go.dev/golang.org/x/oauth2/google#DefaultTokenSource)
to create an
[oauth2.TokenSource](https://pkg.go.dev/golang.org/x/oauth2#TokenSource) to be
used for authenticated access to APIs and resources.

The call to
[google.DefaultTokenSource()](https://pkg.go.dev/golang.org/x/oauth2/google#DefaultTokenSource)
will follow the search algorithm in
[FindDefaultCredentialsWithParams](https://pkg.go.dev/golang.org/x/oauth2/google#FindDefaultCredentialsWithParams).

To run applications locally authenticated as yourself you can run:

```
gcloud auth application-default login
```

Which will place credentials at:

```
$HOME/.config/gcloud/application_default_credentials.json
```

that will be picked up by the application.

If you wish to override that behavior and use a different set of credentials
then set the Environment Variable `GOOGLE_APPLICATION_CREDENTIALS` that points
to a different file, such as a `key.json` file for a specific service account.

When running in kubernetes
[google.DefaultTokenSource()](https://pkg.go.dev/golang.org/x/oauth2/google#DefaultTokenSource)
will pick up credentials from GCP metadata or
[workload identity](http://go/skia-workload-identity).

## Using Git

Use of the Git binary itself is strongly discouraged unless it is unavoidable.
Please consider an alternative:

- go/gitiles provides an API for retrieving commit information, file contents,
  git log, etc, via HTTP for repos hosted on Googlesource.
- go/gitstore provides a low-level interface for retrieving commit metadata by
  time or index. This data is stored in BigTable and is ingested by the
  `gitsync` app, which also sends PubSub messages for low-latency updates.
- go/vcsinfo/bt_vcs provides a similar interface for retrieving metadata but
  adds caching and packages Gitiles into a common API.
- go/git/repograph provides a complete in-memory graph of a repository for fast
  traversal. It loads data via go/gitstore and can be automatically updated via
  PubSub.
- go/gerrit provides access to the Gerrit API, including uploading and
  committing changes to repos which use Gerrit.

The following are valid reasons to use the Git binary itself:

- You need to do more complex write operations, eg. merges.
- You need a full local checkout of some code, eg. to compile and run tests, or
  to run a script. Note that you can use go/gitiles to download a standalone
  script, so a full checkout should not be necessary unless your use case
  requires a large or changing set of files.

If you do need Git for your app, use the `base-cipd` Docker image, which
includes a pinned version of Git (as well as other tools). Do not install Git
via the package manager in your Docker image.

## Launching

- Write/update design doc so that others understand how to use, maintain, and
  improve your service. `DESIGN.md` typically has high level design structures
  (e.g. where is data stored, how do the pieces of software interact, etc).
  `PROD.md` has an overview of the alerts and any other notes for maintaining
  the service.
- Do some back-of-the-envelope calculations to make sure your service can handle
  the expected load. If your service is latency-sensitive, measure the latency
  under load.
- Test on browsers that your users will be using, at least Chrome on desktop and
  ideally Chrome on Android.
- Create an `app.yaml` in
  [k8s-config](https://skia.googlesource.com/k8s-config/+show/master/) This
  controls how your app will be run in GKE. See
  [these docs](https://kubernetes.io/docs/concepts/services-networking/connect-applications-service/)
  for more on the schema. Commit this, then run `pushk appname` to make the
  configuration active.
- Metrics are customarily made available at port 20000. To configure metrics
  scraping the port should be named 'prom'. See [go/skia-infra-metrics](http://go/skia-infra-metrics)
  for more details.

```yaml
ports:
  - containerPort: 20000
    name: prom
```

- Clusters run with [Cluster
  Autoscaler](https://cloud.google.com/kubernetes-engine/docs/concepts/cluster-autoscaler),
  which means that every pod should have the following annotation:

```yaml
annotations:
  cluster-autoscaler.kubernetes.io/safe-to-evict: 'true'
```

If you need finer grained control over how your pods are started and stopped
that can be done by defining a
[PodDisruptionBudget](https://kubernetes.io/docs/concepts/workloads/pods/disruptions/).
CockroachDB defines a PodDisruptionBudget and is a good example of such a
budget.


- Metrics will be available on
  [prom2.skia.org](https://prom2.skia.org/).
- The metrics will be labeled `app=<foo>` where `foo` is the first argument to
  `common.InitWithMust`.
- If you have secrets (like a service account), bind it to the deployment by
  adding the following to `app.yaml`:

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

- If you use OAuth, it should be configured to use the \*.skia.org cookie (the
  default). Additionally, you will need to mount the secrets to use with your
  login.Init\* code:

```yaml
spec:
  ...
  containers:
    - name: my-container
      ...
      volumeMounts:
        - name: skia-org-legacy-login-secrets
          mountPath: /etc/skia.org/
      ...
  volumes:
    - name: skia-org-legacy-login-secrets
      secret:
        secretName: skia-org-legacy-login-secrets
```

- It is possible to test your service/config without making it publicly visible.

  - Deploy your `app.yaml` either with `pushk` or `kubectl apply -f app.yaml`
  - Identify a pod name, `kubectl get pods | grep [my-app]` where my-app is the
    name of the new service.
  - Forward a local port (e.g. 8083) to a port on the pod (e.g. the HTTP port
    8000): `kubectl port-forward my-app-7bf542629-jujzm 8083:8000`
  - Navigate a browser to <http://localhost:8083> to see your service.

- If you have simple routing needs, to make your service visible to the public
  add a
  [`skia.org.domain` annotation to your Service YAML](https://skia.googlesource.com/buildbot/+doc/refs/heads/master/skfe/README.md)
  with the domain name and deploy your updated yaml with `kubectl apply`.

  If your routing is more complicated you can skip the YAML annotation and write
  the routing rules directly into `infra/skfe/k8s/default.conf`.

  Either way you then push a new version of nginx-skia-org:

  ```
  cd infra/skfe
  make k8s_push
  ```

  And watch that the new instances start running:

  ```
  watch kubectl get pods -lapp=nginx-skia-org
  ```

- Add prober rules to `probers.json` in your application directory.

  - Ideally, probe all public HTML pages and all nullipotent JSON endpoints. You
    can write functions in `prober/go/prober/main.go` to check the response body
    if desired.

- Add additional stats gathering to your program using
  `go.skia.org/infra/go/metrics2`, e.g. to ensure liveness/heartbeat of any
  background processes.

- Add alert rules to
  [alerts_public](https://skia.googlesource.com/buildbot/+show/master/promk/prometheus/alerts_public.yml).
  The alerts may link to a production manual, `PROD.md`, checked into the
  application source directory. Examples:

  - All prober rules.
  - Additional stats from metrics2. Legacy apps have their alert rules in
    `prometheus/sys/alert.rules`

- Some
  [general metrics](https://skia.googlesource.com/buildbot/+show/master/promk/prometheus/alerts_general.yml)
  apply to all apps and may not need to be added explicitly for your
  application, such as: - Too many goroutines. - Free disk space on the instance
  and any attached disks. - This is also for alerts that apply to skia-public
  and skia-corp projects.

- Check your alert rules by running `make validate` in `promk/` (Legacy apps
  should run that commaind in `prometheus/`).

- Then, after landing your valid alerts, run
  `make push_config && make push_config_corp` in `promk/` (Again, legacy apps
  should do `make push` in `prometheus/`).

- Tell people about your new service.
- Be prepared for bug reports. :-)

# Continuous Deployment

Some apps are set up to be continuously re-built and re-deployed on every commit
of Skia or Skia Infra. To do that, see
[docker_pushes_watcher/README.md](./docker_pushes_watcher/README.md).
