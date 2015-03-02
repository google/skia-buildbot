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

This means that monit will poll every two seconds that our application is up
and running.


Metadata
========

Secrets that we need at runtime are stored in the metadata server.

The current set of metadata required is:

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

  * skiaperf-com-key, skiaperf-com-pem - The TLS/SSL secrets for skiaperf.com.
    These are stored in the project meta data. See below about managing SSL
    certificates.

  * skiagold-com-key, skiagold-com-pem - The TLS/SSL secrets for skiagold.com.
    These are stored in the project meta data. See below about managing SSL
    certificates.

To set a specific metadata value for an instance use:

    gcloud compute instances add-metadata skia-testing-b \
        --metadata apikey=<apikey value>

This will set the 'apikey' value for the 'skia-testing-b' instance.

To set a specific metadata value on the project level use:

    gcloud compute project-info add-metadata \
        --metadata-from-file skiaperf-com-key="./skiaperf.com.key"

This will set the 'pem' file for skiaperf.com.

See https://cloud.google.com/sdk/gcloud/reference/compute/ for more information
about the 'gcloud' command.


To update the code
==================

1. SSH into the server as default.
2. cd ~/buildbot/perf/setup
3. git pull
4. ./perf_setup.sh

Note: This will also update the SSL secrets (.pem and .key files).


Managing SSL Certificates
=========================

We store the SSL certificates encrypted in a shared folder (shared-skia-infra)
in Google drive. They are in a zip file (skia-infra-ssl-certs.zip.gpg) that is
encrypted with a symmetric key. The key is stored in http://valentine (search
for skiaperf).

To decrypt the file run the following command:

    gpg skia-infra-ssl-certs.zip.gpg

To encrypt a new version of the file file run the following command:

    gpg -c --cipher-algo AES256 skia-infra-ssl-certs.zip

To generate a new encryption key you can use:

    openssl rand -base64 48

which will generate a 48-byte long key and encode it in base64.


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
