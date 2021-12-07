Service Account Keys Checker
============================

The goal of this service is to add metrics for when the keys of the service
accounts in Skia's cloud projects are going to expire, so we can get alerts
based on them.

When dealing with k8s pods/containers/deployments, it is handy to reference:
<https://github.com/skia-dev/textfiles/blob/master/kubectl-cheatsheet.md>

An instance of sa-keys-checker accesses public and corp cloud projects. It
should thus run only in a corp cloud project.

This service requires the "Identity and Access Management" API to be enabled in
the cloud projects it needs to access.

The service account used by this service requires the "Service Account Key
Admin" role in the cloud projects it needs to access.
