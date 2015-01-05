Skia-Buildbot Repository
========================

This repo contains infrastructure code for Skia.


Quick Start
===========

Tests require a local installation of MySQL. For a Debian based distro:
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
