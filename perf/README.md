SkiaPerf Server
===============

Reads Skia performance data from databases and serves interactive dashboards
for easy exploration and annotations.


Server Setup
============

Please refer to compute_engine_scripts/perfserver/README under the repo for
instructions on creating and destroying the instance. The rest of this document
is what to do once the instance is created.


Do the first time
=================

The following things only need to be done once.

1. SSH into the server as default.
2. sudo apt-get install git
3. git clone https://skia.googlesource.com/buildbot
4. cd ~/buildbot/perf/setup
5. ./perf_setup.sh

Change /etc/monit/monitrc to:

    set daemon 2

then run the following so it applies:

    sudo /etc/init.d/monit restart

Then restart squid to pick up the new config file:

    sudo /etc/init.d/squid3 restart

This means that monit will poll every two seconds that our application is up
and running.


Metadata
========

Secrets that we need at runtime are stored in the metadata server.

All of the metadata must be set at one time, i.e. if you change one piece of
metadata you need to write all the values, even the old unchanged metadata
values. The current set of metadata required is:

  * apikey - The API Key found on this page
    https://console.developers.google.com/project/31977622648/apiui/credential
    Used for access to the project hosting API.
  * readwrite - The MySQL readwrite password. Stored in http://valentine,
    search for "skiaperf".
  * cookiesalt - A bit of entropy to use for hashing the users email address
    in the cookie as used for login. Store in http://valentine, search for "skiaperf".
  * clientid and clientsecret - The Client ID and Secret used in the OAuth flow
    for login. The values come from the following page, which is also the
    place to set valid Redirect URLs.

      https://console.developers.google.com/project/31977622648/apiui/credential

To set the metadata use:

    gcutil --project=google.com:skia-buildbots setinstancemetadata skia-testing-b \
      --metadata=apikey:[apikey value] \
      --metadata=readwrite:[readwrite value] \
      --metadata=cookiesalt:[cookiesalt value] \
      --metadata=clientid:[clientid value] \
      --metadata=clientsecret:[clientsecret value] \
      --fingerprint=[metadata fingerprint]

You can find the current metadata fingerprint by running:

    gcutil --project=google.com:skia-buildbots getinstance skia-testing-b

Or you can just use the web UI in the console to set metadata values.


To update the code
==================

1. SSH into the server as default.
2. cd ~/buildbot/perf/setup
3. git pull
4. ./perf_setup.sh


Developing
==========

The easiest way to develop should be to do:

    go get -u skia.googlesource.com/buildbot.git/perf/go/...

Which will fetch the repository into the right location and
download dependencies.

Then to build everything:

    cd $GOPATH/src/skia.googlesource.com/buildbot.git/perf
    make all

Make sure the $GOPATH/bin is on your path so that you can easily run the
executables after they are generated.

To run the tests:

    make test
