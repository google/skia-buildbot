#!/bin/bash
set -e -x

# Compile the Go program.
go install .

DIR=`mktemp -d /tmp/variance-XXXX`

# We are going to run a single process per file, create a directory to store all
# those individual files.
INDIVIDUAL_FILES_DIR="${DIR}/parallel"

mkdir ${INDIVIDUAL_FILES_DIR}

echo "Output directory: ${DIR}"

# Loop over each file and extract data as lines of CSV.
gsutil ls "gs://skia-perf/nano-json-v1/2021/05/25/**" | xargs -P 64 -I{} ./single_step.sh {} ${INDIVIDUAL_FILES_DIR}

# Add column headers.
echo "traceid,mean,median,min,max,ratio"  > ${DIR}/variance.csv

# Concatenate all the individual files into a single CSV file.
cat ${INDIVIDUAL_FILES_DIR}/* >> ${DIR}/variance.csv

# Sort the data based on the ratio column in numerical descending order.
mlr --icsv --ocsv sort -nr ratio ${DIR}/variance.csv > ${DIR}/sorted.csv

# Trim out the top 100 lines from the sorted file, cause Google Sheets can't handle large files.
head --lines=100 ${DIR}/sorted.csv > ${DIR}/short-sorted.csv

echo ${DIR}/short-sorted.csv