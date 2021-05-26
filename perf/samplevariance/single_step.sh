#!/bin/bash

FILE=`mktemp $2/parallel-XXXXXXX`

echo $1 $2
gsutil cat $1 | gunzip | samplevariance > ${FILE}
