#!/bin/bash

set -x

echo "traceid,mean,median,min,max,ratio"  > ./variance.csv

declare -a files
readarray -t files <<< $(gsutil ls "gs://skia-perf/nano-json-v1/2021/05/24/14/**")


for i in "${files[@]}"
do
	echo $i
    gsutil cat $i | gunzip | samplevariance >> ./variance.csv
done

mlr --icsv --ocsv sort -nr ratio variance.csv > sorted.csv
head --lines=100 sorted.csv > short-sorted.csv