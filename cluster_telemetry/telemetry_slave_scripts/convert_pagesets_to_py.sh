# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)
#
# Converts all JSON page_sets in a directory to Python page_sets.


if [ $# -ne 2 ]; then
  echo
  echo "Usage: `basename $0` 1 All"
  echo
  echo "The first argument is the slave_num of this telemetry slave."
  echo "The second argument is the type of pagesets to create from the 1M" \
       "list Eg: All, Filtered, 100k, 10k."
  echo
  exit 1
fi

SLAVE_NUM=$1
PAGESETS_TYPE=$2

source vm_utils.sh

# Download the page_sets from Google Storage if the local TIMESTAMP is out of
# date.
mkdir -p /b/storage/page_sets/$PAGESETS_TYPE/
are_timestamps_equal /b/storage/page_sets/$PAGESETS_TYPE gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE
if [ $? -eq 1 ]; then
  gsutil cp gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/* \
    /b/storage/page_sets/$PAGESETS_TYPE/
fi

function strip_from_val {
  val=$1
  val=${val#\"}
  echo ${val%\"\,}
}

for page_set in /b/storage/page_sets/$PAGESETS_TYPE/*.py; do
  sed -i s/PageWithDefaultRunNavigate/Page/g $page_set
done


# Copy the python page_sets into Google Storage.
gsutil cp /b/storage/page_sets/$PAGESETS_TYPE/*py gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/
# Create a TIMESTAMP file and copy it to Google Storage.
TIMESTAMP=`date +%s`
echo $TIMESTAMP > /tmp/$TIMESTAMP
cp /tmp/$TIMESTAMP /b/storage/page_sets/$PAGESETS_TYPE/TIMESTAMP
gsutil cp /tmp/$TIMESTAMP gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/TIMESTAMP
rm /tmp/$TIMESTAMP

