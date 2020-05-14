goldctl
=======

This command-line tool lets clients upload images to Gold.


Tests
-----

If any interfaces in ./go/ change, it may be necessary to re-generate the
mocks used for testing.

	make mocks


Getting goldctl
---------------

A typical user will download goldctl like any other Go executable

	go install go.skia.org/infra/gold-client/cmd/goldctl

Googlers, if you are using goldctl from Swarming, you'll probably want to
pull it in via [CIPD](https://chrome-infra-packages.appspot.com/p/skia/tools/goldctl)

Deploying to CIPD
-----------------

goldctl is set up to automatically be built and rolled out via
[CIPD](https://chrome-infra-packages.appspot.com/p/skia/tools/goldctl)

To roll a new version, one must update the
[pinned version](https://chromium.googlesource.com/infra/infra/+show/9c99d4cfd6fadf1d53b5cd9a1a2935d03dc67c6a/go/deps.yaml#432)
and follow the procedures for
[updating dependencies](https://chromium.googlesource.com/infra/infra/+show/refs/heads/master/go/#updating-dependencies).

For more details, see:
<https://docs.google.com/document/d/1caPiBgWGjIyOhyMIYYYrFdekzwQ-_nsshp0GWzcSHzA/edit>