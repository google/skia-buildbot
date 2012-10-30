#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Archives or replays webpages and creates skps in a Google Storage location.

To archive webpages and store GM skps (will be run rarely, maybe once a month):

cd .../buildbot/slave/skia_slave_scripts/skpicture_playback
PYTHONPATH=../../../third_party/chromium_trunk/tools/perf:\
../../../third_party/chromium_buildbot/scripts:\
../../../third_party/chromium_buildbot/site_config \
python webpages_playback.py --dest_gsbase=gs://chromium-skia-gm \
--builder_name=Skia_Mac_Float_Bench_64 --record=True


To replay archived webpages and store skps (will be run every CL):

cd .../buildbot/slave/skia_slave_scripts/skpicture_playback
PYTHONPATH=../../../third_party/chromium_trunk/tools/perf:\
../../../third_party/chromium_buildbot/scripts:\
../../../third_party/chromium_buildbot/site_config \
python webpages_playback.py --dest_gsbase=gs://chromium-skia-gm \
--builder_name=Skia_Mac_Float_Bench_64

"""


import optparse
import os
import shutil
import sys
import tempfile

from perf_tools import multipage_benchmark_runner
from slave import slave_utils


# The root directory name will be used both locally and on Google Storage.
ROOT_DIR_NAME = 'skpicture_playback'

# The local root playback directory.
LOCAL_ROOT_PLAYBACK_DIR = os.path.join(tempfile.gettempdir(), ROOT_DIR_NAME)

# The local webpages archive directories.
LOCAL_RECORD_WEBPAGES_ARCHIVE_DIR = os.path.join(
    LOCAL_ROOT_PLAYBACK_DIR, 'webpages_archive')
LOCAL_REPLAY_WEBPAGES_ARCHIVE_DIR = os.path.join(
    os.path.abspath(os.path.dirname(__file__)), os.pardir, os.pardir,
    os.pardir, 'third_party', 'chromium_trunk', 'tools', 'perf', 'data')

# The local skpictures directory.
LOCAL_SKPICTURES_DIR = os.path.join(LOCAL_ROOT_PLAYBACK_DIR, 'skpictures')

# The canned acl to use while copying files to Google Storage.
PLAYBACK_CANNED_ACL = 'project-private'


def main(options):
  page_set = options.page_set
  dest_gsbase = options.dest_gsbase
  builder_name = options.builder_name
  record = options.record
  wpr_file_name = page_set.split('/')[-1].split('.')[0] + '.wpr'

  # Delete the local root directory if it already exists.
  if os.path.exists(LOCAL_ROOT_PLAYBACK_DIR):
    shutil.rmtree(LOCAL_ROOT_PLAYBACK_DIR)   

  # Create the required local storage directories.
  _CreateLocalStorageDirs(record, builder_name)

  if not record:
    # Get the webpages archive from Google Storage so that it can be replayed.
    _DownloadArchiveFromStorage(wpr_file_name, dest_gsbase)

  # Clear all command line arguments and add only the ones supported by
  # the skpicture_printer benchmark.
  _SetupArgsForSkPrinter(page_set, record)

  # Run the skpicture_printer script which:
  # Creates an archive of the specified webpages if '--record' is specified.
  # Saves all webpages in the page_set as skp files.
  multipage_benchmark_runner.Main()

  if record:
    # Move over the created archive into the local webpages archive directory.
    shutil.move(os.path.join(LOCAL_REPLAY_WEBPAGES_ARCHIVE_DIR, wpr_file_name),
                LOCAL_RECORD_WEBPAGES_ARCHIVE_DIR)

  # Delete the local wpr now that we are done with it.
  shutil.rmtree(LOCAL_REPLAY_WEBPAGES_ARCHIVE_DIR);

  # Copy the directory structure in the root directory into Google Storage.
  gs_status = slave_utils.GSUtilCopyDir(
      src_dir=LOCAL_ROOT_PLAYBACK_DIR, gs_base=dest_gsbase,
      dest_dir=ROOT_DIR_NAME, gs_acl=PLAYBACK_CANNED_ACL)
  if gs_status != 0:
    raise Exception('ERROR: GSUtilCopyDir error %d. '
        '"%s" -> "%s/%s"' % (
            gs_status, LOCAL_ROOT_PLAYBACK_DIR, dest_gsbase, ROOT_DIR_NAME))

  return 0


def _SetupArgsForSkPrinter(page_set, record):
  """Setup arguments for the skpicture_printer script.

  Clears all command line arguments and adds only the ones supported by
  skpicture_printer.
  """
  # Clear all command line arguments.
  del sys.argv[:]
  # Dummy first argument.
  sys.argv.append('dummy_file_name')
  if record:
    # Create a new wpr file.
    sys.argv.append('--record')
  # Use the system browser.
  sys.argv.append('--browser=system')
  # Output skp files to skpictures_dir.
  sys.argv.append('--outdir=' + LOCAL_SKPICTURES_DIR)
  # Point to the skpicture_printer benchmark.
  sys.argv.append('skpicture_printer')
  # Point to the top 25 webpages page set.
  sys.argv.append(page_set)


def _CreateLocalStorageDirs(record, builder_name):
  """Creates required local storage directories for this script."""
  global LOCAL_SKPICTURES_DIR

  # Add the builder name to the skp directory.
  LOCAL_SKPICTURES_DIR = os.path.join(LOCAL_SKPICTURES_DIR, builder_name)

  if record:
    os.makedirs(LOCAL_RECORD_WEBPAGES_ARCHIVE_DIR)
    # If in record mode the generated skp files are GM.
    LOCAL_SKPICTURES_DIR = os.path.join(LOCAL_SKPICTURES_DIR, 'gm')

  _CreateCleanLocalDir(LOCAL_SKPICTURES_DIR)
  _CreateCleanLocalDir(LOCAL_REPLAY_WEBPAGES_ARCHIVE_DIR)


def _CreateCleanLocalDir(directory):
  """If directory already exists, it is deleted and recreated."""
  if os.path.exists(directory):
    shutil.rmtree(directory)
  os.makedirs(directory)


def _DownloadArchiveFromStorage(wpr_file_name, dest_gsbase):
  """Download the webpages archive from Google Storage."""
  wpr_source = os.path.join(
      dest_gsbase, ROOT_DIR_NAME, 'webpages_archive', wpr_file_name)
  slave_utils.GSUtilDownloadFile(
      src=wpr_source, dst=LOCAL_REPLAY_WEBPAGES_ARCHIVE_DIR)


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--page_set',
      help='Specifies the page set to use to archive.',
      default=('../../../third_party/chromium_trunk/'
               'tools/perf/page_sets/top_25.json'))
  option_parser.add_option(
      '', '--record',
      help='Specifies whether a new website archive should be created.',
      default=False)  
  option_parser.add_option(
      '', '--dest_gsbase',
      help='gs:// bucket_name, the bucket to upload the file to')
  option_parser.add_option(
      '', '--builder_name',
      help='The name of the builder. Eg: Skia_Mac_Float_Bench_64')
  options, unused_args = option_parser.parse_args()

  sys.exit(main(options))
