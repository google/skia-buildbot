Kubernetes Configuration Templates
==================================

This directory contains templates to generate the boilerplate for creating Gold instances.

To use this, go to the infra/golden repo and run `./gen-k8s-config.sh [name]` where name
is the instance to generate. The configuration of that instance should be in `../k8s-instances`
as a JSON5 file.

There may be some configurations that are checked into:
<https://skia.googlesource.com/infra-internal/+show/refs/heads/master/gold-instance-config/>