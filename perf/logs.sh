#!/bin/bash

gcloud logging read "resource.type=\"container\" logName=\"projects/skia-public/logs/skiaperf-android\" severity>=ERROR timestamp>=\"2019-02-12T20:56:20Z\" timestamp<=\"2019-02-12T20:56:23Z\" " \
  --order=asc --format=json | jq --raw-output .[].textPayload
