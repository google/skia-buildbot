WebTry Server
=============

Allows trying out Skia code in the browser. Run a local webserver
and from the pages it serves try out Skia code and see the results
immediately.


Running Locally
===============

One time setup:

    $ export SKIA_ROOT=path_to_your_skia_source
    $ export WEBTRY_ROOT=path_to_your_webtry_source
    $ export WEBTRY_INOUT=path_to_a_writeable_directory
    $ mkdir -p $WEBTRY_INOUT

Then, to run:

    $ cd $WEBTRY_ROOT
    $ go get -d
    $ ./build
    $ ./webtry

Then visit http://localhost:8000 in your browser.

Only tested under linux and MacOS, doubtful it will work on other platforms.


Server Setup
============

Please refer to compute_engine_scripts/webtry/README under the repo for
instructions on creating and destroying the instance. The rest of this document
is what to do once the instance is created.


Do the first time
=================

The following things only need to be done once.

1. SSH into the server as default.
2. sudo apt-get install git
3. git clone https://skia.googlesource.com/buildbot
4. cd ~/buildbot/webtry/setup
5. ./webtry_setup.sh

6. Add the following to the /etc/schroot/minimal/fstab:

  none /run/shm tmpfs rw,nosuid,nodev,noexec 0 0
  /home/webtry/inout             /skia_build/inout  none    rw,bind         0       0
  /home/webtry/cache             /skia_build/cache  none    rw,bind         0       0

7. Change /etc/monit/monitrc to:

    set daemon 2

then run the following so it applies:

    sudo /etc/init.d/monit restart

This means that monit will poll every two seconds that our application is up and running.

8. Set the TCP keepalive. For more info see:
   https://developers.google.com/cloud-sql/docs/gce-access

    sudo sh -c 'echo 60 > /proc/sys/net/ipv4/tcp_keepalive_time'


To update the code
==================

1. SSH into the server as default.
2. cd ~/buildbot/webtry/setup
3. git pull
4. ./webtry_setup.sh


Third Party Code
================

  * res/js/polyfill.js - Various JS polyfill libraries. To rebuild or update
    see poly/README.md.
