#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Upload a single file to a Google Storage Bucket.

To test:
  cd .../buildbot/slave/skia_slave_scripts/utils
  CR_BUILDBOT_PATH=../../../third_party/chromium_buildbot
  PYTHONPATH=$CR_BUILDBOT_PATH/scripts:$CR_BUILDBOT_PATH/site_config \
  python upload_to_bucket.py \
    --source_filepath=../../../DEPS --dest_gsbase=gs://chromium-skia-gm
"""

from utils import misc
import optparse
import os
import sys

from slave import slave_utils

def upload_to_bucket(source_filepath, dest_gsbase):
  abs_source_filepath = misc.GetAbsPath(source_filepath)
  print 'translated source_filepath %s to absolute path %s' % (
      source_filepath, abs_source_filepath)
  if not os.path.exists(abs_source_filepath):
    raise Exception('ERROR: file not found: %s' % abs_source_filepath)
  status = slave_utils.GSUtilCopyFile(abs_source_filepath, dest_gsbase,
                                      gs_acl='public-read')
  if status != 0:
    raise Exception('ERROR: GSUtilCopyFile error %d. "%s" -> "%s"' % (
        status, abs_source_filepath, dest_gsbase))
  (status, _output) = slave_utils.GSUtilListBucket(dest_gsbase)
  if status != 0:
    raise Exception('ERROR: failed to get list of %s, exiting' % dest_gsbase)
  return 0


def main(argv):
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--source_filepath',
      help='full path of the file we wish to upload')
  option_parser.add_option(
      '', '--dest_gsbase',
      help='gs:// url indicating where to upload the file')
  (options, _args) = option_parser.parse_args()
  return upload_to_bucket(source_filepath=options.source_filepath,
                          dest_gsbase=options.dest_gsbase)


if '__main__' == __name__:
  sys.exit(main(None))
