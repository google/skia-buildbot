#!/bin/bash

# Install all the system level dependencies.
sudo apt-get install --assume-yes apache2 apache2-mpm-worker apache2-utils apache2.2-bin \
  apache2.2-common libapr1 libaprutil1 libaprutil1-dbd-sqlite3 build-essential python-dev \
  libapache2-mod-wsgi libaprutil1-ldap memcached python-cairo-dev python-django python-ldap \
  python-memcache python-pysqlite2 sqlite3 libapache2-mod-python python-pip fontconfig \
  monit

# Now install system dependencies that we need, but aren't satisfiable via
# apt-get.
sudo pip install --upgrade django==1.5
sudo pip install --upgrade django-tagging 'twisted<12.0'
sudo pip install pytz
sudo pip install pyparsing

# Apache runs as the user www-data and we also need the Graphite server, which
# is a WSGI Django application, to run as www-data. Sadly under Debian the
# $HOME directory for www-data is /var/www, which is where files are by
# default served from for Apache, so we can't store any data there securely,
# so we create a /home/www-data directory and install Graphite web and its
# dependencies there.
sudo mkdir /home/www-data
sudo chown www-data:default /home/www-data

# Vars to use with 'install'.
PARAMS="-D --verbose --backup=none --group=default --owner=www-data --preserve-timestamps"
EXE_PARAMS="$PARAMS --mode=766"
CONFIG_PARAMS="$PARAMS  --mode=666"

# Copy over scripts we will run as www-data.
sudo install $EXE_PARAMS continue_install.sh continue_install2.sh /home/www-data

# The continue_install.sh script installs local per-user copies of ceres,
# whisper, carbon and graphite-web.
sudo su www-data -c /home/www-data/continue_install.sh

# Now that the default installs are in place, overwrite the installs with our
# custom config files.
sudo install $CONFIG_PARAMS graphite.wsgi carbon.conf storage-schemas.conf \
  /home/www-data/graphite/conf
sudo install $CONFIG_PARAMS local_settings.py /home/www-data/graphite/lib/graphite/
sudo install $CONFIG_PARAMS monitoring_monit /etc/monit/conf.d/monitoring

# Now run the continue_install2.sh script as www-data, which creates the
# sqlite database if needed and starts the carbon service.
sudo su www-data -c /home/www-data/continue_install2.sh

# Add to the configuration of Apache so that we load the Graphite WSGI
# application, and restart the server.
sudo cp httpd.conf /etc/apache2/conf.d/graphite.conf
sudo /etc/init.d/monit restart
sudo /etc/init.d/apache2 restart
