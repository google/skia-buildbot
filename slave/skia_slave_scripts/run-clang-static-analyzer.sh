#!/bin/bash
#
# Runs the clang static analyzer on trunk and stores its results in a directory
# in Skia's Google Storage.
# By default the results are stored here:
# https://storage.cloud.google.com/chromium-skia-gm/static_analyzers/clang_static_analyzer/index.html
#
# Sample Usage:
#  STATIC_ANALYZER_TEMPDIR=/tmp/clang-static-analyzer \
#  SKIA_GOOGLE_STORAGE_BASE=gs://chromium-skia-gm \
#  bash update-buildbot-pydoc.sh


# Initialize and validate environment variables.
STATIC_ANALYZER_TEMPDIR=${STATIC_ANALYZER_TEMPDIR:-/tmp/clang-static-analyzer}
SKIA_GOOGLE_STORAGE_BASE=${SKIA_GOOGLE_STORAGE_BASE:-gs://chromium-skia-gm}
GOOGLE_STORAGE_CLANG_URL="https:\/\/storage.cloud.google.com\/chromium-skia-gm\/static_analyzers\/clang_static_analyzer\/"

if [[ $SKIA_GOOGLE_STORAGE_BASE =~ ^gs://.* ]]; then
  if [[ "$SKIA_GOOGLE_STORAGE_BASE" =~ ^gs://.+/.+ ]]; then
    echo -e "\n\nPlease only specify the Google Storage base to use. Eg: gs://chromium-skia-gm. Provided value: $SKIA_GOOGLE_STORAGE_BASE"
    exit 1
  fi
else
  echo -e "\n\nSKIA_GOOGLE_STORAGE_BASE must start with gs://. Provided value: $SKIA_GOOGLE_STORAGE_BASE"
  exit 1
fi

SKIA_GOOGLE_STORAGE_STATIC_ANALYSIS_DIR=${SKIA_GOOGLE_STORAGE_BASE}/static_analyzers/clang_static_analyzer


# Clean and create the temporary directory.
if [ -d "$STATIC_ANALYZER_TEMPDIR" ]; then
  rm -rf $STATIC_ANALYZER_TEMPDIR
fi
mkdir -p $STATIC_ANALYZER_TEMPDIR


# Run the clang static analyzer.
# Details about the analyzer are here: http://clang-analyzer.llvm.org/.
scan-build -o $STATIC_ANALYZER_TEMPDIR make

ret_code=$?
if [ $ret_code != 0 ]; then
  echo "Error while executing the scan-build command"
  exit $ret_code
fi


# Fix static file paths to point to storage.cloud.google.com else the files will not be found.
sed -i 's/href="\(.*\.css\)/href="'$GOOGLE_STORAGE_CLANG_URL'\1/g' $STATIC_ANALYZER_TEMPDIR/*/index.html
sed -i 's/src="\(.*\.js\)/src="'$GOOGLE_STORAGE_CLANG_URL'\1/g' $STATIC_ANALYZER_TEMPDIR/*/index.html
sed -i 's/href="\(.*\.html\)/href="'$GOOGLE_STORAGE_CLANG_URL'\1/g' $STATIC_ANALYZER_TEMPDIR/*/index.html


# Clean the SKIA_GOOGLE_STORAGE_STATIC_ANALYSIS_DIR.
gsutil rm ${SKIA_GOOGLE_STORAGE_STATIC_ANALYSIS_DIR}/*
# Copy analysis results to SKIA_GOOGLE_STORAGE_STATIC_ANALYSIS_DIR.
gsutil cp -a public-read $STATIC_ANALYZER_TEMPDIR/*/* $SKIA_GOOGLE_STORAGE_STATIC_ANALYSIS_DIR/


echo -e "\n\nThe scan-build results are available to view here: https://storage.cloud.google.com/${SKIA_GOOGLE_STORAGE_STATIC_ANALYSIS_DIR:5}/index.html"

