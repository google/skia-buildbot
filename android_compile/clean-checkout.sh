#! /bin/bash

# Bash script to clean an android checkout.

if [ -z "$1" ]
  then
    echo "Missing Android checkout directory"
    exit 1
fi
checkout=$1
cd $checkout

find . -name shallow.lock -exec echo "Going to delete " {} \; -exec rm {} \;
