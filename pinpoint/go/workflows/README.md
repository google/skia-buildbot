# How to run temporal workflow

## Run temporal workflow on dev

1. Connect your cloudtop to GKE cluster:
   `./kube/attach.sh skia-infra-public-dev`
2. Connect to the service at localhost 7233:<br>
   `kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233`
3. (To test local changes) Run the worker with namespace perf-internal and taskqueue localhost.dev:
   `bazelisk run //pinpoint/go/workflows/worker -- --namespace perf-internal`
4. (if needed) Change workflow parameters in `sample/sample.go`
5. Trigger the workflow:<br>
   `bazelisk run  //pinpoint/go/workflows/sample -- --namespace=perf-internal --bisect=true`

- make sure the namespace and taskqueue matches
- there are command line flags for which workflow you want to trigger

6. Check the workflow status on the dev deployment
   [here](https://temporal-ui-dev.corp.goog/namespaces/perf-internal/workflows).

- Note that workflows disappear from the landing page after 24 hours.
- Can also view results from the
  [archival page](https://temporal-ui-dev.corp.goog/namespaces/perf-internal/archival)

You need to redo steps 3-5 if you want to run your latest local changes.

## Run temporal workflow on prod (follow these steps with care)

1. Skia breakglass (More context at go/skia-infra-iac-handbook under `Breakglass Access`):<br>
   `grants add --wait_for_twosync --reason="b/XYZ -- justification"
skia-infra-breakglass-policy:2h`
2. Connect your cloudtop to GKE cluster belonging to one of the production instances:
   - `./kube/attach.sh skia-infra-corp`
   - `./kube/attach.sh skia-infra-prod`
3. Connect to the service at localhost 7233:<br>
   `kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233`
4. (if needed) Change workflow parameters in `sample/sample.go`
5. Trigger the workflow:<br>
   `bazelisk run  //pinpoint/go/workflows/sample -- --namespace=perf-internal
--taskQueue=perf.perf-chrome-public.bisect --[flag_for_workflow i.e. bisect]=true`
6. Check the workflow status:
   - [skia-infra-corp](https://skia-temporal-ui.corp.goog/namespaces/perf-internal/workflows)
   - [skia-infra-prod](https://temporal-ui.skia.org/namespaces/perf-internal/workflows)

Notes:

- For breakglass, adding a bug and a justification is required. Breakglass access is self-approved.
- Temporal will by default use the prod workers when connecting to `skia-infra-public` so
  connecting to the worker is unnecessary.

# Troubleshooting

## 403 to chrome-swarming

Please ask the team to add you to
https://chrome-infra-auth.appspot.com/auth/groups/project-chromeperf-admins.

# Common Temporal development pitfalls

- All parameters must be exportable for golang to serialize the parameter into
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
