#!/bin/bash
set -e

# Compile the Go program.
go install .

DIR=`mktemp -d /tmp/variance-XXXX`

echo "Output directory: ${DIR}"

# Add column headers.
echo "traceid,mean,median,min,max,ratio"  > ${DIR}/variance.csv

# Create an array with all the files to process. Requires read access to the
# Google Cloud Storage bucket.
declare -a files
readarray -t files <<< $(gsutil ls "gs://skia-perf/nano-json-v1/2021/05/24/14/**")

# Loop over each file and extract data as lines of CSV.
for i in "${files[@]}"
do
	echo $i
    gsutil cat $i | gunzip | samplevariance >> ${DIR}/variance.csv
done

# Sort the data based on the ratio column in numerical descending order.
mlr --icsv --ocsv sort -nr ratio ${DIR}/variance.csv > ${DIR}/sorted.csv

# Trim out the top 100 lines from the sorted file, cause Google Sheets can't handle large files.
head --lines=100 ${DIR}/sorted.csv > ${DIR}/short-sorted.csv

echo ${DIR}/short-sorted.csv