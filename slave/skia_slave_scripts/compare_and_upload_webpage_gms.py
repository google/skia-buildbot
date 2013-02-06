#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Compares the GM images from archived webpages to the baselines.

This module can be run from the command-line like this:

cd buildbot/third_party/chromium_buildbot/slave/\
Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64/build/trunk

PYTHONPATH=../../../../site_config:\
../../../../scripts \
python ../../../../../../slave/skia_slave_scripts/\
compare_and_upload_webpage_gms.py \
--configuration "Debug" --target_platform "" --revision 0 \
--autogen_svn_baseurl "" --make_flags "" --test_args "" --gm_args "" \
--bench_args "" --num_cores 8 --perf_output_basedir "" \
--builder_name Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64 \
--got_revision 0 --gm_image_subdir base-shuttle_ubuntu12_ati5770 \
--is_try False --do_upload_results True --dest_gsbase gs://rmistry

"""

import os
import posixpath
import shutil
import sys
import tempfile

from utils import gs_utils
from utils import shell_utils
from utils import sync_bucket_subdir
from build_step import PLAYBACK_CANNED_ACL
from build_step import BuildStep, BuildStepWarning

import build_step

SKP_TIMEOUT_MULTIPLIER = 8
LAST_COMPARISON_FILENAME = 'LAST_COMPARISON_SUCCEEDED'
REBASELINE_IN_PROGRESS_FILENAME = 'REBASELINE_IN_PROGRESS'

GM_COMPARISON_LINES_TO_EXTRACT_IMAGES = [
  'have identical dimensions but some differing pixels',
  'have differing dimensions',
  'not found in baseDir and found in comparisonDir',
]

GM_COMPARISON_LINES_TO_DELETE_IMAGES = [
  'found in baseDir and not found in comparisonDir',
]

IMAGES_FOR_UPLOAD_CHUNKS = [
  'base-macmini',
  'base-macmini-lion-float',
  'base-shuttle_ubuntu12_ati5770',
]


class CompareAndUploadWebpageGMs(BuildStep):

  def __init__(
      self,
      timeout=build_step.DEFAULT_TIMEOUT * SKP_TIMEOUT_MULTIPLIER,
      no_output_timeout=(
          build_step.DEFAULT_NO_OUTPUT_TIMEOUT * SKP_TIMEOUT_MULTIPLIER),
      **kwargs):
    """Constructs a RenderWebpagePictures BuildStep instance.

    timeout: maximum time allowed for this BuildStep. The default value here is
             increased because there could be a lot of skps' whose images have
             to be copied over to Google Storage.
    no_output_timeout: maximum time allowed for this BuildStep to run without
        any output.
    """
    build_step.BuildStep.__init__(self, timeout=timeout,
                                  no_output_timeout=no_output_timeout,
                                  **kwargs)

    self._dest_gsbase = (self._args.get('dest_gsbase') or
                         sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)

    self._upload_chunks = (
        True if self._gm_image_subdir in IMAGES_FOR_UPLOAD_CHUNKS
             else False)

    # Check if gm-expected exists on Google Storage.
    self._gm_expected_exists_on_storage = gs_utils.DoesStorageObjectExist(
        posixpath.join(self._dest_gsbase,
                       self._storage_playback_dirs.PlaybackGmExpectedDir(),
                       gs_utils.TIMESTAMP_COMPLETED_FILENAME))
    # Check if gm-actual exists on Google Storage.
    self._gm_actual_exists_on_storage = gs_utils.DoesStorageObjectExist(
        posixpath.join(self._dest_gsbase,
                       self._storage_playback_dirs.PlaybackGmActualDir(),
                       gs_utils.TIMESTAMP_COMPLETED_FILENAME))

  def _Run(self):
    cmd = [self._PathToBinary('skdiff'),
           '--listfilenames',
           '--nodiffs',
           '--nomatch', gs_utils.TIMESTAMP_STARTED_FILENAME,
           '--nomatch', gs_utils.TIMESTAMP_COMPLETED_FILENAME,
           '--nomatch', gs_utils.LAST_REBASELINED_BY_FILENAME,
           '--nomatch', LAST_COMPARISON_FILENAME,
           '--failonresult', 'DifferentPixels',
           '--failonresult', 'DifferentSizes',
           '--failonresult', 'DifferentOther',
           '--failonresult', 'CouldNotCompare',
           '--failonresult', 'Unknown',
           self._local_playback_dirs.PlaybackGmExpectedDir(),
           self._local_playback_dirs.PlaybackGmActualDir(),
           ]

    # Temporary list of builders who are allowed to fail this step without the
    # bot turning red.
    may_fail_with_warning = [
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Debug_32',
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Debug_32_Trybot',
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_32',
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_32_Trybot',
        'Skia_Shuttle_Win7_Intel_Float_Debug_64',
        'Skia_Shuttle_Win7_Intel_Float_Debug_64_Trybot',
        'Skia_Shuttle_Win7_Intel_Float_Release_64',
        'Skia_Shuttle_Win7_Intel_Float_Release_64_Trybot',
        'Skia_Mac_Float_Debug_64',
        'Skia_Mac_Float_Debug_64_Trybot',
        'Skia_Mac_Float_Release_64',
        'Skia_Mac_Float_Release_64_Trybot',
        'Skia_MacMiniLion_Float_Debug_64',
        'Skia_MacMiniLion_Float_Debug_64_Trybot',
        'Skia_MacMiniLion_Float_Release_64',
        'Skia_MacMiniLion_Float_Release_64_Trybot',
        ]

    # Fail with a warning if the gm_actual directory on Google Storage is
    # currently being rebaselined else running this Build step will result in
    # data being in an inconsistent state.
    if gs_utils.DoesStorageObjectExist(
        posixpath.join(self._dest_gsbase,
                       self._storage_playback_dirs.PlaybackGmActualDir(),
                       REBASELINE_IN_PROGRESS_FILENAME)):
      raise Exception(
          'Rebaselining is currently in progress. Failing this build step since'
          ' running it could result in data being in an inconsistent state. See'
          ' https://codereview.appspot.com/7069066/ (Adding a Lock file to'
          ' prevent rebaseline script clashing with the build steps) for'
          ' details.')

    if not self._gm_expected_exists_on_storage:
      # Copy images to expected directory if gm-expected has not been created in
      # Storage yet.
      print '\n\n=========Copying gm-actual to gm-expected locally=========\n\n'
      if os.path.exists(self._local_playback_dirs.PlaybackGmExpectedDir()):
        shutil.rmtree(self._local_playback_dirs.PlaybackGmExpectedDir())
      shutil.copytree(self._local_playback_dirs.PlaybackGmActualDir(),
                      self._local_playback_dirs.PlaybackGmExpectedDir())
    else:
      print '\n\n=======Downloading gm-expected from Google Storage=======\n\n'
      gs_utils.DownloadDirectoryContentsIfChanged(
          gs_base=self._dest_gsbase,
          gs_relative_dir=self._storage_playback_dirs.PlaybackGmExpectedDir(),
          local_dir=self._local_playback_dirs.PlaybackGmExpectedDir())

    if not self._gm_actual_exists_on_storage and self._do_upload_results:
      # Copy actual images to Google Storage since they do not exist yet.
      print '\n\n========Uploading gm-actual to Google Storage========\n\n'
      gs_utils.UploadDirectoryContentsIfChanged(
          gs_base=self._dest_gsbase,
          gs_relative_dir=self._storage_playback_dirs.PlaybackGmActualDir(),
          gs_acl=PLAYBACK_CANNED_ACL,
          local_dir=self._local_playback_dirs.PlaybackGmActualDir(),
          upload_chunks=self._upload_chunks)

    # Debugging statements.
    print '\n\n=======Directory Contents=======\n\n'
    expected_contents = os.listdir(
        self._local_playback_dirs.PlaybackGmExpectedDir())
    print 'Contents of gm-expected:'
    print expected_contents
    print len(expected_contents)

    actual_contents = os.listdir(
        self._local_playback_dirs.PlaybackGmActualDir())
    print '\n\nContents of gm-actual:'
    print actual_contents
    print len(actual_contents)

    skp_contents = os.listdir(
        self._local_playback_dirs.PlaybackSkpDir())
    print '\n\nContents of skp dir:'
    print skp_contents
    print len(skp_contents)

    gm_comparison_output = ''
    last_comparison_successful = self._ReadFromLastComparisonFile() == 'True'
    try:
      print '\n\n=========Running GM Comparison=========\n\n'
      proc = shell_utils.BashAsync(cmd, echo=True, shell=False)
      (returncode, gm_comparison_output) = shell_utils.LogProcessToCompletion(
          proc, echo=True, timeout=None)
      if returncode != 0:
        raise Exception('Command failed with code %d' % returncode)
    except Exception as e:
      print '\n\n=========GM Comparison Failed!=========\n\n'
      if self._do_upload_results and self._gm_actual_exists_on_storage:

        gm_expected_timestamp_newer = self._isGMExpectedTimestampNewer()
        if gm_expected_timestamp_newer or last_comparison_successful:
          # Logging statements.
          if gm_expected_timestamp_newer:
            print '\n\n======gm-expected timestamp is newer than gm-actual====='
          if last_comparison_successful:
            print '\n\n======Last GM Comparison was successful======'
          print '======Uploading gm-actual to Google Storage======\n\n'

          # Try to get the list of changed images by parsing the
          # gm_comparison_output. If we cannot determine which images have
          # changed then we upload all images.
          images_list = []
          images_to_delete_list = []
          for output_line in gm_comparison_output.split('\n'):
            for gm_comparison_line in GM_COMPARISON_LINES_TO_EXTRACT_IMAGES:
              if gm_comparison_line in output_line:
                images_list.extend([image for image in output_line.split()
                                    if 'png' in image])
            for gm_comparison_line in GM_COMPARISON_LINES_TO_DELETE_IMAGES:
              if gm_comparison_line in output_line:
                images_to_delete_list.extend(
                    [image for image in output_line.split() if 'png' in image])

          if images_to_delete_list:
            gs_utils.DeleteDirectoryContents(
                gs_base=self._dest_gsbase,
                gs_relative_dir=(
                    self._storage_playback_dirs.PlaybackGmActualDir()),
                files_to_delete=images_to_delete_list)

          if not images_to_delete_list or (
              images_to_delete_list and images_list):
            gs_utils.UploadDirectoryContentsIfChanged(
                gs_base=self._dest_gsbase,
                gs_relative_dir=(
                    self._storage_playback_dirs.PlaybackGmActualDir()),
                gs_acl=PLAYBACK_CANNED_ACL,
                local_dir=self._local_playback_dirs.PlaybackGmActualDir(),
                force_upload=True,
                upload_chunks=self._upload_chunks,
                files_to_upload=images_list)

      print '\n\nUpdate the gm-actual local LAST_COMPARISON_SUCCEEDED'
      self._WriteToLastComparisonFile(False)

      print '\n\n=========Raising the GM Comparison Error=========\n\n'
      if self._builder_name in may_fail_with_warning:
        raise BuildStepWarning(e)
      else:
        # TODO(rmistry): Change the below line to 'raise' after
        # https://code.google.com/p/skia/issues/detail?id=1027 is fixed.
        raise BuildStepWarning(e)
    else:
      print '\n\n=========GM Comparison Succeeded!=========\n\n'
      if (self._do_upload_results and self._gm_actual_exists_on_storage and
          not last_comparison_successful):
        print '\n\n======Last GM Comparison was unsuccessful======'
        print '======Uploading gm-actual to Google Storage======\n\n'
        gs_utils.UploadDirectoryContentsIfChanged(
            gs_base=self._dest_gsbase,
            gs_relative_dir=self._storage_playback_dirs.PlaybackGmActualDir(),
            gs_acl=PLAYBACK_CANNED_ACL,
            local_dir=self._local_playback_dirs.PlaybackGmActualDir(),
            force_upload=True,
            upload_chunks=self._upload_chunks)       
 
      print 'Update the gm-actual local LAST_COMPARISON_SUCCEEDED'
      self._WriteToLastComparisonFile(True)

  def _isGMExpectedTimestampNewer(self):
    """Compares timestamps from gm-expected and gm-actual.

    Returns true iff the timestamp in this platform's gm-expected directory
    in Google Storage was uploaded more recently than its gm-actual directory.
    """
    gs_actual_timestamp = gs_utils.ReadTimeStampFile(
        timestamp_file_name=gs_utils.TIMESTAMP_COMPLETED_FILENAME,
        gs_base=self._dest_gsbase,
        gs_relative_dir=self._storage_playback_dirs.PlaybackGmActualDir())
    gs_expected_timestamp = gs_utils.ReadTimeStampFile(
        timestamp_file_name=gs_utils.TIMESTAMP_COMPLETED_FILENAME,
        gs_base=self._dest_gsbase,
        gs_relative_dir=self._storage_playback_dirs.PlaybackGmExpectedDir())
    return gs_actual_timestamp < gs_expected_timestamp
    
  def _WriteToLastComparisonFile(self, value):
    comparison_file = os.path.join(tempfile.gettempdir(),
                                   LAST_COMPARISON_FILENAME)
    f = open(comparison_file, 'w')
    try:
      f.write(str(value))
    finally:
      f.close()
    shutil.copyfile(
        comparison_file,
        os.path.join(self._local_playback_dirs.PlaybackGmActualDir(),
                     LAST_COMPARISON_FILENAME))

  def _ReadFromLastComparisonFile(self):
    """Returns 'True' if the file does not exist."""
    comparison_file = os.path.join(
        self._local_playback_dirs.PlaybackGmActualDir(),
        LAST_COMPARISON_FILENAME)
    if not os.path.exists(comparison_file):
      return 'True'
    f = open(comparison_file, 'r')
    try:
      value = f.read()
      return value.strip()
    finally:
      f.close()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CompareAndUploadWebpageGMs))
