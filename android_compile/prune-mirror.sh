#! /bin/bash

# Bash script to prune git gc from all projects. This is done to prevent the
# mirror syncs from gradually slowing down over time. See skbug.com/8053 for
# more context.

if [ -z "$1" ]
  then
    echo "Missing Android checkout directory"
    exit 1
fi
checkout=$1
cd $checkout

find . -type d -name ".git" -execdir nice -n 19 ionice -c3 git gc --aggressive --prune \;
