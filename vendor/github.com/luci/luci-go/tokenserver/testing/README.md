# The token server <-> client semi-manual integration test

This test sets up and configures local instance of the token server and tests
that clients are able to use it.

Prerequisites:

* Some Cloud Project that will host service account. You must have "Editor" and
  "Service account actor" roles in this project, and Cloud IAM API must be
  enabled. The test will create `luci-token-server-test-1` service account
  belonging to this project.
* [gcloud](https://cloud.google.com/sdk/gcloud/) tool, with you authenticated
  (via `gcloud init`). It magically enables GAE dev server to use real OAuth
  tokens (belonging to your account) when making URL fetch calls (in particular
  to Cloud IAM).
* `$GOROOT` and `$GOPATH` properly configured, `$GOBIN` in `$PATH`.
* `openssl` tool available in `$PATH`.
* `gae.py` tool available in `$PATH`.

How to run the test:

1. Open a terminal tab, run `devserver.sh`. It will start the token server.
1. In another tab run `clrserver.sh`. It will start a server that serves CRLs.
1. Finally, in the main tab run `run_test.sh` to execute the actual test.

Keeping other two services in separate tabs is helpful for two reasons:

* You can investigate the state of the server using RPC explorer in the browser.
* Starting and killing background devappserver in a script is surprisingly
  difficult task. `Ctrl+C` in a tab seems to do the trick more reliably.
