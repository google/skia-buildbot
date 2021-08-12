Powercycle Controller
=====================

The purpose of this module is to expose a web UI for powercycling various machines in
the Skia fleet.

<http://go/skolo-powercycle>

See also `//skolo/go/powercycle/DESIGN.md` for a more up-to-date design.

Local Testing
-------------

The easiest way to test locally is to set up an SSH tunnel to skia-prom

    gcloud compute ssh --zone us-central1-c --project google.com:skia-buildbots skia-prom -- -v -L 8001:skia-prom:8001

Then, start the server,

    go run ./go/power-controller/main.go --alerts_endpoint localhost:8001 --local --powercycle_config ../skolo/sys/powercycle.json5
