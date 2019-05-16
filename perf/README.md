SkiaPerf Server
===============

Reads Skia performance data from databases and serves interactive dashboards
for easy exploration and annotations.


Server Setup
============

Please refer to compute_engine_scripts/perf/README under the repo for
instructions on creating and destroying the instance. The rest of this
document is what to do once the instance is created.


Metadata
========

Secrets that we need at runtime are stored in the metadata server.

The current set of project level metadata required is:

  * apikey - The API Key found on this page
    https://console.developers.google.com/project/31977622648/apiui/credential
    Used for access to the project hosting API.
  * cookiesalt - A bit of entropy to use for hashing the users email address
    in the cookie as used for login. Store in http://valentine, search for "skiaperf".
  * clientid and clientsecret - The Client ID and Secret used in the OAuth flow
    for login. The values come from the following page, which is also the
    place to set valid Redirect URLs.

      https://console.developers.google.com/project/31977622648/apiui/credential


Developing
==========

The easiest way to develop should be to do:

    go get -u go.skia.org/infra/perf/go/...

Which will fetch the repository into the right location and
download dependencies.

Then to build everything:

    cd $GOPATH/src/go.skia.org/infra/perf
    make all

Make sure the $GOPATH/bin is on your path so that you can easily run the
executables after they are generated.

To run the tests:

    make test
