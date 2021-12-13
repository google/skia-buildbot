# General
proberk runs in kubernetes and there is no HTML frontend. The prober configuration is baked into
the Docker container - in order to update that, one must rebuild and re-deploy the container.

# Alerts

## ProbeFailure
This alert means that one of the `ResponseTester`s is not happy with the result. The prober name
should be in the alert message and can help explain more precisely what is going wrong. In all
cases, looking at the logs from kubernetes can be helpful in learning more. The alert message
should have a search link to help identify the file which defines the prober.

### gobPotentialRepoLeak
This ProberFailure type means that the current list of public repos visible at
<https://skia.googlesource.com/?format=JSON> does not match the baked in version. This could mean
we accidentally leaked something private, but more often simply means we just added a GoB mirror
(e.g. go/new-skia-git-mirror).

To address this, run `make update-expectations` from //proberk, verify the diffs, check it in,
and re-deploy. This will update //proberk/expectations/gob.json

