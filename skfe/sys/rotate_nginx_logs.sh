#!/bin/bash

# This script renames the current nginx logfiles and sends a USR1 signal to
# nginx to re-open the log.

LOG_DIR="/var/log/nginx"
NGINX_PID="/var/run/nginx.pid"
rm -f $LOG_DIR/*.log.1
rename 's/.log$/.log.1/' *
kill -USR1 `cat $NGINX_PID`
