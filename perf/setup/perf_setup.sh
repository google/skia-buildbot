#!/bin/bash
#
# Script to setup a GCE instance to run the perf server.
# For full instructions see the README file.

# Command to download metadata.
CURL_PROJ='curl -H "Metadata-Flavor: Google"'

sudo apt-get install monit nginx gcc mercurial make nodejs nodejs-legacy
echo "Adding the perf user account"
sudo adduser perf

PARAMS="-D --verbose --backup=none --group=default --owner=perf --preserve-timestamps"

sudo install $PARAMS --mode=766 continue_install /home/perf/
sudo install $PARAMS --mode=666 -T sys/_bash_aliases /home/perf/.bash_aliases
sudo su perf -c /home/perf/continue_install

sudo cp sys/perf_init /etc/init.d/skiaperf
sudo chmod 744 /etc/init.d/skiaperf
sudo cp sys/ingest_init /etc/init.d/ingest
sudo chmod 744 /etc/init.d/ingest
sudo cp sys/logserver_init /etc/init.d/logserver
sudo chmod 744 /etc/init.d/logserver
sudo cp sys/perf_monit /etc/monit/conf.d/perf
sudo cp sys/correctness_init /etc/init.d/skiacorrectness
sudo chmod 744 /etc/init.d/skiacorrectness

# Add the nginx configuration files
sudo cp sys/perf_nginx /etc/nginx/sites-available/perf
sudo rm -f /etc/nginx/sites-enabled/perf
sudo ln -s /etc/nginx/sites-available/perf /etc/nginx/sites-enabled/perf

sudo cp sys/gold_nginx /etc/nginx/sites-available/gold
sudo rm -f /etc/nginx/sites-enabled/gold
sudo ln -s /etc/nginx/sites-available/gold /etc/nginx/sites-enabled/gold

# Download the SSL secrets from the metadata store.
sudo $CURL_META http://metadata/computeMetadata/v1/project/attributes/skiagold-com-key > /etc/nginx/ssl/skiagold_com.key
sudo $CURL_META http://metadata/computeMetadata/v1/project/attributes/skiagold-com-pem > /etc/nginx/ssl/skiagold_com.pem
sudo $CURL_META http://metadata/computeMetadata/v1/project/attributes/skiaperf-com-key > /etc/nginx/ssl/skiaperf_com.key
sudo $CURL_META http://metadata/computeMetadata/v1/project/attributes/skiaperf-com-pem > /etc/nginx/ssl/skiaperf_com.pem
chmod 700 /etc/nginx/ssl
chmod 600 /etc/nginx/ssl/*

# Confirm that monit is happy.
sudo monit -t
sudo monit reload

sudo /etc/init.d/skiaperf restart
sudo /etc/init.d/ingest restart
sudo /etc/init.d/logserver restart
sudo /etc/init.d/skiacorrectness restart
sudo /etc/init.d/nginx status
