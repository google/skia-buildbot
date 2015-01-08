Skia-Buildbot Repository
========================

This repo contains infrastructure code for Skia.


Getting the Source Code
=======================

The main source code repository is a Git repository hosted at
https://skia.googlesource.com/buildbot. Although it is possible to check out
this repository directly with git clone, buildbot dependencies are managed by
gclient instead of Git submodules, so to work on buildbot, it is better to
install [depot_tools](http://www.chromium.org/developers/how-tos/install-depot-tools).

Initial Checkout

$ mkdir ~/skia_infra
$ cd ~/skia_infra
$ fetch skia_buildbot

Subsequent Checkouts

$ cd ~/skia_infra/buildbot
$ git pull --rebase
$ gclient sync


Go Setup
========

For working on Go code run:

$ go get -u skia.googlesource.com/buildbot.git/...

This fetches the repository into your $GOPATH directory along with all the
needed dependencies.


Quick Start
===========

Go tests require a local installation of MySQL. For a Debian based distro:
$ sudo apt-get install mysql-client mysql-server

Then, to set up local versions of the production databases:
$ cd go/database
$ ./setup_test_db


Database Setup for Testing
==========================

Tests which use the database package's testutils require you to have a MySQL
instance running with a database named "sk_testing" and users called
"readwrite" and "test_root" with appropriate permissions for sk_testing.
The 'setup_test_db' script in 'go/database' is included for convenience in
setting up this test database and user.
