#!/bin/bash
#
# Create all GCE VM firewalls.
#
# Copyright 2013 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)

source vm_config.sh

$GCOMPUTE_CMD addfirewall allow-admin-http-default --allowed="tcp:10115" \
  --print_json --description="Incoming admin http allowed on 10115" \
  --allowed_ip_sources="216.239.32.0/19,64.233.160.0/19,66.249.80.0/20,72.14.192.0/18,209.85.128.0/17,66.102.0.0/20,74.125.0.0/16,64.18.0.0/20,207.126.144.0/20,173.194.0.0/16,74.202.227.225"

$GCOMPUTE_CMD addfirewall allow-http-slaves --allowed="tcp:10116" --print_json \
  --description="Incoming http allowed on 10116 for slaves"

$GCOMPUTE_CMD addfirewall allow-http-default --allowed="tcp:10117" \
  --print_json --description="Incoming http allowed on 10117" --network=default

exit 0
