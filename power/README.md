Powercycle Controller
=====================

The purpose of this module is to expose a web UI for powercycling various machines in
the Skia fleet.

https://docs.google.com/document/d/15sb7RN_S3ctw06xQoNG7c3Owu-DS2jPPDaWAdQIkPLw/edit#


Local Testing
-------------

The easiest way to test locally is to set up an SSH tunnel to skia-prom

    gcloud compute ssh --zone us-central1-c --project google.com:skia-buildbots skia-prom -- -v -L 8001:skia-prom:8001

Then, start the server,

    go run ./go/power-controller/main.go --logtostderr --alerts_endpoint localhost:8001 --local --powercycle_config ../skolo/sys/powercycle.yaml