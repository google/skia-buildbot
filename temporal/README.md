# Temporal

[Temporal](https://github.com/temporalio/temporal) is an execution service,
serving jobs orchestration, [see alternatives](go/qa-alter).

# Docker Images

The binaries are being built in go/louhi and the source codes are pulled from the
individual releases.

## Temporal Server [Releases](https://github.com/temporalio/temporal/releases)

There are three binaries:

- temporal-server: the main server binary
- temporal-sql-too: the database tool to initialize and upgrade schemas
- tdbg: the debugging utils

## Temporal CLI [Releases](https://github.com/temporalio/cli/releases)

The CLI tool to admin the service:

`bazelisk run //temporal:temporal-cli --`

This assumes the service is running locally at port 7233.

## Temporal UI Server [Release](https://github.com/temporalio/ui-server/releases)

The Web UI frontend to inspect the service.

# How Tos

## Run Temporal Workflow

Note that these instructions use the pinpoint workflows, namespace, and task queue as examples.
The default dev task queue is localhost.dev. The namespace is perf-internal.

### Locally trigger temporal workflow in dev env

Follow these steps to test local changes.
You need to redo steps 3-5 if you want to run your latest local changes.

1. Connect your cloudtop to GKE cluster:<br>
   `./kube/attach.sh skia-infra-public-dev`
2. Connect to the service at localhost 7233:<br>
   `kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233`
3. Create a new terminal and run the worker. Make sure the taskqueue and namespace are correct:<br>
   `bazelisk run //pinpoint/go/workflows/worker -- --namespace perf-internal`
4. (if needed) Change workflow parameters in `sample/main.go`
5. Create a new terminal and trigger the workflow:<br>
   `bazelisk run  //pinpoint/go/workflows/sample -- --namespace=perf-internal --[flag]=true`
6. Check the workflow status on the dev deployment
   [here](https://temporal-ui-dev.corp.goog/namespaces/perf-internal/workflows).

After results expire from the landing page, you can view results from the
[archival page](https://temporal-ui-dev.corp.goog/namespaces/perf-internal/archival)

### Locally trigger production workflow (follow these steps with care)

You cannot test local changes this way. These steps are for testing things that
can only be tested in prod.

1. Break glass (More context at [go/skia-infra-iac-handbook](go/skia-infra-iac-handbook)
   under Breakglass Access):<br>
   `grants add --wait_for_twosync --reason="b/XYZ -- justification"
skia-infra-breakglass-policy:2h`
2. Connect your cloudtop to GKE cluster belonging to one of the production instances:
   - `./kube/attach.sh skia-infra-corp`
   - `./kube/attach.sh skia-infra-public`
3. Connect to the service at localhost 7233:<br>
   `kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233`
4. (if needed) Change workflow parameters in `sample/sample.go`
5. Create a new terminal and trigger the workflow.
   Make sure the taskqueue and namespace are correct:<br>
   `bazelisk run  //pinpoint/go/workflows/sample -- --namespace=perf-internal
--taskQueue=perf.perf-chrome-public.bisect --[flag_for_workflow i.e. bisect]=true`
6. Check the workflow status. Your workflow may be located in a different namespace:
   - [skia-infra-corp](https://skia-temporal-ui.corp.goog/namespaces/perf-internal/workflows)
   - [skia-infra-public](https://temporal-ui.skia.org/namespaces/perf-internal/workflows)

Notes:

- For breakglass, adding a bug and a justification is required. Breakglass access requires a +1.
- Temporal will by default use the prod workers when connecting to prod env so
  connecting to the worker is unnecessary.

## Namespace (follow these steps with care)

### Create a namespace

1. Break glass (More context at [go/skia-infra-iac-handbook](go/skia-infra-iac-handbook)
   under Breakglass Access):<br>
   `grants add --wait_for_twosync --reason="b/XYZ -- justification" skia-infra-breakglass-policy:2h`
2. Connect your cloudtop to GKE cluster:<br>
   `./kube/attach.sh [name of cluster i.e. skia-infra-corp]`
3. Connect to the service at localhost 7233:<br>
   `kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233`
4. Once connected, create a new terminal. Run the following from skia root:<br>
   `bazelisk run @com_github_temporal_cli//:temporal-bins/temporal --
operator namespace create --retention=[duration] --history-archival-state=enabled
--visibility-archival-state=enabled [name of namespace]`

Where the retention is how long the workflow shows up in history. The default is 72 hours.

### Update temporal namespace

For updating name spaces, follow the instructions above to connect to the cluster.

```
// to get help
bazelisk run @com_github_temporal_cli//:temporal-bins/temporal -- operator namespace update -h

// updating the retention period
bazelisk run @com_github_temporal_cli//:temporal-bins/temporal --
operator namespace update --retention [duration] [namespace name]
```

# Troubleshooting

## Collect event history for workflow test replay

Temporal workflows uses event history replay to ensure that any changes to a workflow will not
break any existing running workflow (called non-determinism error). Proper versioning is required
to avoid non-determinism errors and replay unit tests confirm versioning is successful without
deploying the change to production or running the workflow in real time.

Here is how to download the event history from a temporal workflow:

### Via the temporal workflow UI

On a given temporal workflow page, you can download the event history by clicking the `Download`
button to the right of the `Event History` header. Disable `Decode Event History`.

This method is the fastest way to get the event history. This method may break if there is a version
incompatibility between the temporal UI and the server. If this occurs, either upgrade temporal
until they are both compatible or try the command line method.

### Via the temporal command line

1. Connect to the dev or prod instance and turn on port forwarding as described above.
2. Trigger the command line:
   `bazelisk run //temporal:temporal-cli -- workflow show -w <workflow-id> -n
<namespace> -o json > <filename>`

You should see the event history in `<filename>`.

## 403 to chrome-swarming

Please ask the team to add you to
https://chrome-infra-auth.appspot.com/auth/groups/project-chromeperf-admins.

## WriteBisectToCatapultActivity: `The catapult post request failed with status code 500`

Only applies to running the Pinpoint catapult bisect workflow on the local dev environment.

- Check if the bisection covered any non-chromium commits
- See if those repositories are omitted
  [here](https://pantheon.corp.google.com/datastore/databases/-default-/entities;kind=Repository;ns=__$DEFAULT$__/query/kind?e=-13802955&mods=component_inspector&project=chromeperf-stage)
- If they are omitted, then temporal will be unable to write the UI result to staging.
- Disable the activity.

# Common Temporal development pitfalls

All parameters must be exportable for golang to serialize the parameter into
a temporal workflow or activity. i.e.

```
type example struct {
   key string
}
param := example{
   key: "value"
}
workflow.ExecuteActivity(ctx, FooActivity, param)
```

`param` will be empty.
