#!/bin/bash
set -e -x

# Calculate the default set of JSON files we'll analyze, which is all the files
# from yesterday.
YESTERDAY=`date --utc +%Y/%m/%d --date=yesterday`
DEFAULT="gs://skia-perf/nano-json-v1/${YESTERDAY}/**"

# A command line argument will override the default.
if [ $# -eq 1 ]; then
DEFAULT=$1
fi

echo ${DEFAULT}

# Store all the output in a temporary directory.
DIR=`mktemp -d /tmp/variance-XXXX`

echo "Output directory: ${DIR}"

# We are going to run a single process per file, create a directory to store all
# those individual files.
INDIVIDUAL_FILES_DIR="${DIR}/parallel"
mkdir ${INDIVIDUAL_FILES_DIR}

# Loop over each file and extract data as lines of CSV. The xargs -P NN causes
# NN parallel exections of single_step, which greatly speeds up the process.
gsutil ls "${DEFAULT}" | xargs -P 64 -I{} ./perf/samplevariance/single_step.sh {} ${INDIVIDUAL_FILES_DIR}

# Add column headers.
echo "traceid,mean,median,min,max,ratio"  > ${DIR}/variance.csv

# Concatenate all the individual files from the parallel runs into a single CSV
# file.
cat ${INDIVIDUAL_FILES_DIR}/* >> ${DIR}/variance.csv

# Sort the data based on the ratio column in numerical descending order.
mlr --icsv --ocsv sort -nr ratio ${DIR}/variance.csv > ${DIR}/sorted.csv

# Trim out the top 100 lines from the sorted file, cause Google Sheets can't handle large files.
head --lines=100 ${DIR}/sorted.csv > ${DIR}/short-sorted.csv

# Print the full name of the most useful file we've generated.
echo ${DIR}/short-sorted.csv