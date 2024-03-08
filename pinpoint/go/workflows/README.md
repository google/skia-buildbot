# How to run temporal workflow

1. Connect your cloudtop to GKE cluster:
   `./kube/attach.sh skia-infra-public-dev`
2. Connect to the service at localhost 7233:
   `kubectl port-forward service/temporal --address 0.0.0.0 -n temporal 7233:7233`
3. (To test local changes) Run the worker under namespace perf-internal and taskqueue localhost.dev:
   `bazelisk run //pinpoint/go/workflows/worker -- --namespace perf-internal`
4. Change workflow parameters in `sample/sample.go`
5. Trigger the workflow:
   `bazelisk run  //pinpoint/go/workflows/sample -- --namespace=perf-internal --bisect=true`

- make sure the namespace and taskqueue matches
- there are command line flags for which workflow you want to trigger

6. Check the workflow status
   [here](https://temporal-ui-dev.corp.goog/namespaces/perf-internal/workflows).

- Note that workflows disappear from the landing page after 24 hours.
- Can also view results from the
  [archival page](https://temporal-ui-dev.corp.goog/namespaces/perf-internal/archival)

You need to redo steps 3-5 if you want to run your latest local changes.

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
