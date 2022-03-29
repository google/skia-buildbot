Kubernetes Checker Production Manual
====================================

The goal of this service is to monitor our Kubernetes (k8s) cluster and pipe relevant metrics into
Prometheus, so we can get alerts based on them.

When dealing with k8s pods/containers/deployments, it is handy to reference:
<https://github.com/skia-dev/textfiles/blob/master/kubectl-cheatsheet.md>

This app syncs https://skia.googlesource.com/k8s-config/ and uses it to generate some metrics.

One instance of k8s-checker should run on each production GCE cluster where we run our services.
As of now, we do not run it in any skolo clusters.

Alerts
======

Items below here should include target links from alerts.

K8sCheckerLiveness
------------------
This alert signifies that the k8s-checker service is running, but has failed to update the metrics
about its cluster recently. It normally attempts to do so once per minute.

Check out the logs of the relevant k8s-checker service.

If the data for this alert is missing, that probably means k8s-checker is not running.

Key metrics: liveness_k8s_checker_s

TooManyPodRestarts
------------------
This alert triggers if a pod has restarted many times since it was deployed. This can indicate a
rare crash (e.g. nil dereference) or a burst of restarts due to an external dependency outage.

To gather more information, use `kubectl logs -f <pod_name>` to get the current logs from the
container (to see if it is currently running ok) and `kubectl logs -f <pod_name> --previous`
to attempt to ascertain the cause of the previous restarts.

`kubectl describe <pod_name> | grep -A 5 "Last State"` can also give information about the previous
life of the pod (e.g "Reason: Error" or "Reason: OOMKilled").

Ways to address the alert include deploying a new (hopefully fixed) version of the container or
explicitly re-deploying the current version to reset the restart count metric. Contact the service
owner to discuss the best mitigation approach.

Key metrics: pod_restart_count

PodRestartingFrequently
-----------------------
This alert triggers if a pod has restarted multiple times in the last hour. This can indicate a
currently down or crash-looping service.

The same advice as the TooManyPodRestarts alert applies.

Key metrics: pod_restart_count

EvictedPod
----------
A pod has been evicted, commonly for using too much memory.

To get the reason, try `k describe pod <pod_name> | grep -A 4 "Status"`. Contact the service owner
with this reason, file a bug if necessary, and then clean up the Evicted pod with
`kubectl delete pod <pod_name> --grace-period=0 --force`

Key metrics: evicted_pod_metric

DirtyCommittedK8sImage
----------------------
A dirty image has been commited to the prod checkout prod checkout. Check with the service owner
and/or the image author to see if they are done experimenting and if we can land/push a clean image.

Key metrics: dirty_committed_image_metric

DirtyRunningK8sConfig
---------------------
A dirty image has been running in production for at least two hours. Check with the service owner
and/or the image author to see if they are done experimenting and if we can land/push a clean image.

Key metrics: dirty_config_metric

StaleK8sImage
-------------
The same k8s image has been running in production for at least 30 days. We should push an updated
image soon to pick up new changes and ensure things continue to work (and aren't secretly broken
for weeks).

Contact the service owner to see if the image can be updated.

Key metrics: stale_image_metric

CheckedInK8sAppNotRunning
-------------------------
An app has a checked in .yaml file, but it currently is not running in production. This might mean
that a service owner forgot to push it after checking it in, or somehow it has stopped being run.

Check with the service owner to see if it needs to be deployed.

Known exceptions:
 - Gold has some test server configs for doing integration tests of goldpushk. These shouldn't be
   running unless those tests are being run manually.

Key metrics: app_running_metric

CheckedInK8sContainerNotRunning
-------------------------------
A container exists in a checked in .yaml file, but it currently is not running in production.
This might mean that a service owner forgot to push it after checking it in, or somehow it has
stopped being run.

Check with the service owner to see if it needs to be deployed.

Key metrics: container_running_metric

RunningK8sAppNotCheckedIn
-------------------------
An app is running in production, but does not belong to a checked in .yaml file.

This typically happens if someone is testing out a new service. Reach out to them for more details.

Key metrics: running_app_has_config_metric

RunningK8sContainerNotCheckedIn
-------------------------------
A container is running in production, but does not belong to a checked in .yaml file.

This typically happens if someone is testing out a new service. Reach out to them for more details.

Key metrics: running_container_has_config_metric

AppRunningInDefaultNamespace
----------------------------
An app is running in the default namespace. This is undesirable for
organizational reasons and because we use namespaces as security boundaries.  If
this app was pushed to the default namespace by accident, delete it and re-push
to the correct namespace.  Otherwise, create a namespace following the example
of the other apps in the cluster.

Key metrics: pod_running
