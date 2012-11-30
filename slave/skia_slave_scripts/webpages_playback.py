#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Archives or replays webpages and creates skps in a Google Storage location.

To archive webpages and store skp files (will be run rarely):

cd ../buildbot/slave/skia_slave_scripts
PYTHONPATH=../../third_party/chromium_trunk/tools/perf:\
../../third_party/src/third_party/webpagereplay:\
../../third_party/chromium_trunk/tools/telemetry:\
../../third_party/chromium_buildbot/scripts:\
../../third_party/chromium_buildbot/site_config \
python webpages_playback.py --dest_gsbase=gs://rmistry \
--record=True


To replay archived webpages and re-generate skp files (will be run whenever
SkPicture.PICTURE_VERSION changes):

cd ../buildbot/slave/skia_slave_scripts
PYTHONPATH=../../third_party/chromium_trunk/tools/perf:\
../../third_party/src/third_party/webpagereplay:\
../../third_party/chromium_trunk/tools/telemetry:\
../../third_party/chromium_buildbot/scripts:\
../../third_party/chromium_buildbot/site_config \
python webpages_playback.py --dest_gsbase=gs://rmistry

"""

import cPickle
import optparse
import os
import posixpath
import shutil
import sys
import tempfile
import traceback

from perf_tools import skpicture_printer
from slave import slave_utils
from telemetry import multi_page_benchmark_runner
from telemetry import wpr_modes
from telemetry import user_agent
from utils import file_utils
from utils import gs_utils

from build_step import PLAYBACK_CANNED_ACL
from playback_dirs import ROOT_PLAYBACK_DIR_NAME
from playback_dirs import SKPICTURES_DIR_NAME


# Local archive and skp directories.
LOCAL_PLAYBACK_ROOT_DIR = os.path.join(
    tempfile.gettempdir(), ROOT_PLAYBACK_DIR_NAME)
LOCAL_REPLAY_WEBPAGES_ARCHIVE_DIR = os.path.join(
    os.path.abspath(os.path.dirname(__file__)), 'page_sets', 'data')
LOCAL_RECORD_WEBPAGES_ARCHIVE_DIR = os.path.join(
    tempfile.gettempdir(), ROOT_PLAYBACK_DIR_NAME, 'webpages_archive')
LOCAL_SKP_DIR = os.path.join(
    tempfile.gettempdir(), ROOT_PLAYBACK_DIR_NAME, SKPICTURES_DIR_NAME)

# Number of times we retry telemetry if there is a problem.
NUM_TIMES_TO_RETRY = 5

# The max base name length of Skp files.
MAX_SKP_BASE_NAME_LEN = 31

# Add Nexus10 to the UA_TYPE_MAPPING list in telemetry.user_agent.
user_agent.UA_TYPE_MAPPING['nexus10'] = (
    'Mozilla/5.0 (Linux; Android 4.2; Nexus 10 Build/JOP40C) '
    'AppleWebKit/535.19 (KHTML, like Gecko) Chrome/18.0.1025.166 '
    'Safari/535.19')

# Dictionary of page_sets to platform prefixes for skp files to help identify
# which platform they were generated for.
PAGE_SET_TO_PLATFORM_PREFIX = {
    'skia_desktop_set.json': 'desk',
    'skia_galaxynexus_set.json': 'mobi',
    'skia_nexus10_set.json': 'tabl',
}


class SkPicturePlayback(object):
  """Class that archives or replays webpages and creates skps."""

  def __init__(self, parse_options):
    """Constructs a SkPicturePlayback BuildStep instance."""
    self._page_set = parse_options.page_set
    self._dest_gsbase = parse_options.dest_gsbase
    self._record = parse_options.record
    self._wpr_file_name = self._page_set.split('/')[-1].split('.')[0] + '.wpr'

  def Run(self):
    """Run the SkPicturePlayback BuildStep."""

    # Delete the local root directory if it already exists.
    if os.path.exists(LOCAL_PLAYBACK_ROOT_DIR):
      shutil.rmtree(LOCAL_PLAYBACK_ROOT_DIR)

    # Create the required local storage directories.
    self._CreateLocalStorageDirs()

    if not self._record:
      # Get the webpages archive from Google Storage so that it can be replayed.
      self._DownloadArchiveFromStorage()

    # Clear all command line arguments and add only the ones supported by
    # the skpicture_printer benchmark.
    self._SetupArgsForSkPrinter()

    # Adding retries to workaround the bug
    # https://code.google.com/p/chromium/issues/detail?id=161244.
    num_times_retried = 0
    retry = True
    while retry:
      # Run the skpicture_printer script which:
      # Creates an archive of the specified webpages if '--record' is specified.
      # Saves all webpages in the page_set as skp files.
      benchmark_dir = os.path.join(
          os.path.abspath(os.path.dirname(__file__)), os.pardir, os.pardir,
          'third_party', 'chromium_trunk', 'tools', 'perf', 'perf_tools',)
      multi_page_benchmark_runner.Main(benchmark_dir)

      try:
        cPickle.load(open(os.path.join(
            LOCAL_REPLAY_WEBPAGES_ARCHIVE_DIR, self._wpr_file_name), 'rb'))
        retry = False
      except EOFError, e:
        traceback.print_exc()
        num_times_retried += 1
        if num_times_retried > NUM_TIMES_TO_RETRY:
          print 'Exceeded number of times to retry!'
          raise e
        else:
          print 'Retrying!'

    if self._record:
      # Move over the created archive into the local webpages archive directory.
      shutil.move(
          os.path.join(LOCAL_REPLAY_WEBPAGES_ARCHIVE_DIR, self._wpr_file_name),
          LOCAL_RECORD_WEBPAGES_ARCHIVE_DIR)

    # Rename generated skp files into more descriptive names.
    self._RenameSkpFiles()

    # Copy the directory structure in the root directory into Google Storage.
    gs_status = slave_utils.GSUtilCopyDir(
        src_dir=LOCAL_PLAYBACK_ROOT_DIR, gs_base=self._dest_gsbase,
        dest_dir=ROOT_PLAYBACK_DIR_NAME, gs_acl=PLAYBACK_CANNED_ACL)
    if gs_status != 0:
      raise Exception(
          'ERROR: GSUtilCopyDir error %d. "%s" -> "%s/%s"' % (
              gs_status, LOCAL_PLAYBACK_ROOT_DIR, self._dest_gsbase,
              ROOT_PLAYBACK_DIR_NAME))
    
    # Add a TIMESTAMP file to the skp directory in Google Storage so we can use
    # directory level rsync like functionality.
    gs_utils.WriteCurrentTimeStamp(
        gs_base=self._dest_gsbase,
        dest_dir=posixpath.join(ROOT_PLAYBACK_DIR_NAME, SKPICTURES_DIR_NAME),
        gs_acl=PLAYBACK_CANNED_ACL)
    
    return 0

  def _RenameSkpFiles(self):
    """Rename generated skp files into more descriptive names.

    All skp files are currently called layer_X.skp where X is an integer, they
    will be renamed into http_website_name_X.skp.

    Eg: http_news_yahoo_com/layer_0.skp -> http_news_yahoo_com_0.skp
    """
    for (dirpath, unused_dirnames, filenames) in os.walk(LOCAL_SKP_DIR):
      if not dirpath or not filenames:
        continue
      basename = os.path.basename(dirpath)
      for filename in filenames:
        filename_parts = filename.split('.')
        extension = filename_parts[1]
        integer = filename_parts[0].split('_')[1]
        if integer != '0':
          # We only care about layer 0s.
          continue
        basename = basename.rstrip('_')

        # Replace the prefix http/https with the platform prefix.
        basename = basename.replace(
            basename.split('_')[0],
            PAGE_SET_TO_PLATFORM_PREFIX[self._page_set.split('/')[-1]],
            1)

        # Ensure the basename is not too long.
        if len(basename) > MAX_SKP_BASE_NAME_LEN:
          basename = basename[0:MAX_SKP_BASE_NAME_LEN]
        new_filename = '%s.%s' % (basename, extension)
        shutil.move(os.path.join(dirpath, filename),
                    os.path.join(LOCAL_SKP_DIR, new_filename))
      shutil.rmtree(dirpath)

  def AddSkPicturePrinterOptions(self, parser):
    """Temporary workaround for a chromium bug.
    
    skpicture_printer.SkPicturePrinter has AddOptions but it should instead have
    AddCommandLineOptions so it can override
    page_test.PageTest.AddCommandLineOptions.
    """
    parser.add_option('--record', action='store_const',
                      dest='wpr_mode', const=wpr_modes.WPR_RECORD,
                      help='Record to the page set archive')
    parser.add_option('-o', '--outdir', help='Output directory',
                      default=LOCAL_SKP_DIR)

  def CustomizeBrowserOptions(self, options):
    """Specifying Skia specific browser options."""
    options.extra_browser_args.extend(['--enable-gpu-benchmarking',
                                       '--no-sandbox',
                                       '--force-compositing-mode'])
    

  def _SetupArgsForSkPrinter(self):
    """Setup arguments for the skpicture_printer script.

    Clears all command line arguments and adds only the ones supported by
    skpicture_printer.
    """
    # Clear all command line arguments.
    del sys.argv[:]
    # Dummy first argument.
    sys.argv.append('dummy_file_name')
    if self._record:
      # Create a new wpr file.
      sys.argv.append('--record')
    # Use the system browser.
    sys.argv.append('--browser=system')
    # Specify extra browser args needed for Skia.
    skpicture_printer.SkPicturePrinter.CustomizeBrowserOptions = (
        self.CustomizeBrowserOptions)
    # Output skp files to skpictures_dir.
    skpicture_printer.SkPicturePrinter.AddCommandLineOptions = (
        self.AddSkPicturePrinterOptions)

    # Point to the skpicture_printer benchmark.
    sys.argv.append('skpicture_printer')
    # Point to the top 25 webpages page set.
    sys.argv.append(self._page_set)

  def _CreateLocalStorageDirs(self):
    """Creates required local storage directories for this script."""
    file_utils.CreateCleanLocalDir(LOCAL_RECORD_WEBPAGES_ARCHIVE_DIR)
    file_utils.CreateCleanLocalDir(LOCAL_SKP_DIR)

  def _DownloadArchiveFromStorage(self):
    """Download the webpages archive from Google Storage."""
    wpr_source = posixpath.join(
        self._dest_gsbase, ROOT_PLAYBACK_DIR_NAME, 'webpages_archive',
        self._wpr_file_name)
    slave_utils.GSUtilDownloadFile(
        src=wpr_source, dst=LOCAL_REPLAY_WEBPAGES_ARCHIVE_DIR)


if '__main__' == __name__:
  option_parser = optparse.OptionParser()
  option_parser.add_option(
      '', '--page_set',
      help='Specifies the page set to use to archive.',
      default=(
          os.path.join(os.path.abspath(os.path.dirname(__file__)),
                       'page_sets', 'skia_desktop_set.json')))
  option_parser.add_option(
      '', '--record',
      help='Specifies whether a new website archive should be created.',
      default=False)
  option_parser.add_option(
      '', '--dest_gsbase',
      help='gs:// bucket_name, the bucket to upload the file to',
      default='gs://chromium-skia-gm')
  options, unused_args = option_parser.parse_args()

  playback = SkPicturePlayback(options)
  sys.exit(playback.Run())
