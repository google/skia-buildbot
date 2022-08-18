# static_server

Static server runs in skia-corp which can't use workload identity, so there is a
`rotate-skia-eskia-sa.sh` script which can be run to update the service account
key used by the application.