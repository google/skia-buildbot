# Copyright 2014 Google Inc. All Rights Reserved.
# Author: rmistry@google.com (Ravi Mistry)
#
# Converts all JSON page_sets in a directory to Python page_sets.


if [ $# -ne 2 ]; then
  echo
  echo "Usage: `basename $0` All"
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

for page_set in /b/storage/page_sets/$PAGESETS_TYPE/*.json; do
  archive_data_file=`awk '/\"archive_data_file\"/{print $NF}' $page_set`
  archive_data_file=`strip_from_val $archive_data_file`

  url=`awk '/\"url\"/{print $NF}' $page_set`
  url=`strip_from_val $url`

  rank=`cat $page_set | grep \"why\"\: | awk '{FS=":"; print $2}'`
  rank=`strip_from_val $rank`

  file_name=`basename $page_set`
  file=${file_name%.json}

  cat >/b/storage/page_sets/$PAGESETS_TYPE/$file.py <<EOL
# Copyright 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.
# pylint: disable=W0401,W0614

from telemetry.page.actions.all_page_actions import *
from telemetry.page import page as page_module
from telemetry.page import page_set as page_set_module


class TypicalAlexaPage(page_module.PageWithDefaultRunNavigate):

  def __init__(self, url, page_set):
    super(TypicalAlexaPage, self).__init__(url=url, page_set=page_set)
    self.user_agent_type = 'desktop'
    self.archive_data_file = '$archive_data_file'

  def RunSmoothness(self, action_runner):
    action_runner.RunAction(ScrollAction())


class TypicalAlexaPageSet(page_set_module.PageSet):

  """ Pages designed to represent the median, not highly optimized web """

  def __init__(self):
    super(TypicalAlexaPageSet, self).__init__(
      user_agent_type='desktop',
      archive_data_file='$archive_data_file')

    urls_list = [
      # Why: Alexa games $rank
      '$url',
    ]

    for url in urls_list:
      self.AddPage(TypicalAlexaPage(url, self))
EOL

done


# Copy the python page_sets into Google Storage.
gsutil cp /b/storage/page_sets/$PAGESETS_TYPE/*py gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/
# Create a TIMESTAMP file and copy it to Google Storage.
TIMESTAMP=`date +%s`
echo $TIMESTAMP > /tmp/$TIMESTAMP
cp /tmp/$TIMESTAMP /b/storage/page_sets/$PAGESETS_TYPE/TIMESTAMP
gsutil cp /tmp/$TIMESTAMP gs://chromium-skia-gm/telemetry/page_sets/slave$SLAVE_NUM/$PAGESETS_TYPE/TIMESTAMP
rm /tmp/$TIMESTAMP

