#!/bin/bash
#
# Script to setup a GCE instance to run the perf server.
# For full instructions see the README file.
sudo apt-get install monit squid3 gcc mercurial make nodejs nodejs-legacy
echo "Adding the perf user account"
sudo adduser perf

PARAMS="-D --verbose --backup=none --group=default --owner=perf --preserve-timestamps"

sudo install $PARAMS --mode=766 continue_install /home/perf/
sudo install $PARAMS --mode=666 -T sys/_bash_aliases /home/perf/.bash_aliases
sudo su perf -c /home/perf/continue_install

sudo cp sys/perf_init /etc/init.d/perf
sudo chmod 744 /etc/init.d/perf
sudo cp sys/tilebuilder_init /etc/init.d/tilebuilder
sudo chmod 744 /etc/init.d/tilebuilder
sudo cp sys/ingest_init /etc/init.d/ingest
sudo chmod 744 /etc/init.d/ingest
sudo cp sys/logserver_init /etc/init.d/logserver
sudo chmod 744 /etc/init.d/logserver
sudo cp sys/perf_monit /etc/monit/conf.d/perf
sudo cp sys/perf_squid /etc/squid3/squid.conf
# Confirm that monit is happy.
sudo monit -t
sudo monit reload
sudo /etc/init.d/perf restart
sudo /etc/init.d/tilebuilder restart
sudo /etc/init.d/ingest restart
sudo /etc/init.d/logserver restart
