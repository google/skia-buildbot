#!/bin/bash
#
# Script to setup a GCE instance to run the perf server.
# For full instructions see the README file.

sudo apt-get install --assume-yes monit nginx gcc mercurial make nodejs nodejs-legacy gcc
echo "Adding the perf user account"
sudo adduser perf

# Create the data directory and make 'perf' the owner.
DATA_DIR="/mnt/pd0/data"
sudo mkdir -p $DATA_DIR
sudo chown perf:perf $DATA_DIR

PARAMS="-D --verbose --backup=none --group=default --owner=perf --preserve-timestamps"
ROOT_PARAMS="-D --verbose --backup=none --group=root --owner=root --preserve-timestamps -T"
EXE_FILE="--mode=755"
CONFIG_FILE="--mode=666"

# Install pull.
# Temporary step to bootstrap monitoring using Skia Push.
gsutil cp gs://skia-push/debs/pull/pull:jcgregorio@jcgregorio.cnc.corp.google.com:2014-12-15T14:12:52Z:6152bc3bcdaa54989c957809e77bed282c35676b.deb pull.deb
sudo dpkg -i pull.deb

sudo install $PARAMS $EXE_FILE continue_install /home/perf/
sudo install $PARAMS $CONFIG_FILE -T sys/_bash_aliases /home/perf/.bash_aliases
sudo su perf -c /home/perf/continue_install

sudo install $ROOT_PARAMS $EXE_FILE sys/perf_init /etc/init.d/skiaperf
sudo install $ROOT_PARAMS $EXE_FILE sys/correctness_init /etc/init.d/skiacorrectness
sudo install $ROOT_PARAMS $EXE_FILE sys/ingest_init /etc/init.d/ingest
sudo install $ROOT_PARAMS $CONFIG_FILE sys/perf_monit /etc/monit/conf.d/perf

# Make sure the log direcotry for nginx exists.
sudo mkdir -p /mnt/pd0/wwwlogs
sudo chown www-data:www-data /mnt/pd0/wwwlogs

# Add the nginx configuration files.
sudo cp sys/perf_nginx /etc/nginx/sites-available/perf
sudo rm -f /etc/nginx/sites-enabled/perf
sudo ln -s /etc/nginx/sites-available/perf /etc/nginx/sites-enabled/perf

sudo cp sys/gold_nginx /etc/nginx/sites-available/gold
sudo rm -f /etc/nginx/sites-enabled/gold
sudo ln -s /etc/nginx/sites-available/gold /etc/nginx/sites-enabled/gold

sudo cp sys/skbug_nginx /etc/nginx/sites-available/skbug
sudo rm -f /etc/nginx/sites-enabled/skbug
sudo ln -s /etc/nginx/sites-available/skbug /etc/nginx/sites-enabled/skbug

# Download the SSL secrets from the metadata store.
CURL_CMD='curl -H "Metadata-Flavor: Google"'
META_PROJ_URL='http://metadata/computeMetadata/v1/project/attributes'

sudo mkdir -p /etc/nginx/ssl/
sudo sh <<CURL_SCRIPT
    $CURL_CMD $META_PROJ_URL/skiagold-com-key -o /etc/nginx/ssl/skiagold_com.key
    $CURL_CMD $META_PROJ_URL/skiagold-com-pem -o /etc/nginx/ssl/skiagold_com.pem
    $CURL_CMD $META_PROJ_URL/skiaperf-com-key -o /etc/nginx/ssl/skiaperf_com.key
    $CURL_CMD $META_PROJ_URL/skiaperf-com-pem -o /etc/nginx/ssl/skiaperf_com.pem
CURL_SCRIPT
sudo chmod 700 /etc/nginx/ssl
sudo sh -c "chmod 600 /etc/nginx/ssl/*"

# Confirm that monit is happy.
sudo monit -t
sudo monit reload

sudo /etc/init.d/skiaperf restart
sudo /etc/init.d/skiacorrectness restart
sudo /etc/init.d/ingest restart
sudo /etc/init.d/nginx restart
