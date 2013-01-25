#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Downloads a single file from a Google Storage Bucket.

To test:
  cd .../buildbot/slave/skia_slave_scripts/utils
  CR_BUILDBOT_PATH=../../../third_party/chromium_buildbot
  PYTHONPATH=$CR_BUILDBOT_PATH/scripts:$CR_BUILDBOT_PATH/site_config \
  python download_from_bucket.py \
    --source_gsurl=gs://chromium-skia-gm/DEPS --dest=~/DEPS
"""

import optparse
import sys

from slave import slave_utils


def DownloadFromBucket(source_gsurl, dest):
  status = slave_utils.GSUtilDownloadFile(source_gsurl, dest)
  if status != 0:
    raise Exception('ERROR: GSUtilDownloadFile error %d. "%s" -> "%s"' % (
                    status, source_gsurl, dest))
  return 0


def main(argv):
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--source_gsurl',
      help='gs://url of the file to download')
  option_parser.add_option(
      '', '--dest',
      help='Destination file/directory where the file will be downloaded.')
  (options, _args) = option_parser.parse_args()
  return DownloadFromBucket(source_gsurl=options.source_gsurl,
                            dest=options.dest)


if '__main__' == __name__:
  sys.exit(main(None))
