#!/bin/bash

# Does the work of a single file from run.sh.
#
# $1 = Name of file in Google Cloud Storage to process.
# $2 = Directory where to store the intermediate CSV file.

# Store the output in a unique file since we are run as part of a group of
# parallel processes.
FILE=`mktemp $2/parallel-XXXXXXX`

echo $1
gsutil cat $1 | gunzip | samplevariance > ${FILE}
