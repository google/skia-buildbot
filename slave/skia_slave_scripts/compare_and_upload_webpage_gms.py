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
--do_upload_results True --dest_gsbase gs://rmistry

"""

import os
import posixpath
import shutil
import sys

from utils import file_utils
from utils import gs_utils
from utils import shell_utils
from utils import sync_bucket_subdir
from build_step import PLAYBACK_CANNED_ACL
from build_step import BuildStep, BuildStepWarning

import build_step

SKP_TIMEOUT_MULTIPLIER = 5


class CompareAndUploadWebpageGMs(BuildStep):

  def __init__(
      self, args, attempts=1,
      timeout=build_step.DEFAULT_TIMEOUT * SKP_TIMEOUT_MULTIPLIER,
      no_output_timeout=(
          build_step.DEFAULT_NO_OUTPUT_TIMEOUT * SKP_TIMEOUT_MULTIPLIER)):
    """Constructs a RenderWebpagePictures BuildStep instance.

    args: dictionary containing arguments to this BuildStep.
    attempts: how many times to try this BuildStep before giving up.
    timeout: maximum time allowed for this BuildStep. The default value here is
             increased because there could be a lot of skps' whose images have
             to be copied over to Google Storage.
    no_output_timeout: maximum time allowed for this BuildStep to run without
        any output.
    """
    build_step.BuildStep.__init__(self, args, attempts, timeout,
                                  no_output_timeout)

    self._dest_gsbase = (self._args.get('dest_gsbase') or
                         sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)

    # Check if gm-expected exists on Google Storage.
    self._gm_expected_exists_on_storage = gs_utils.DoesStorageObjectExist(
        posixpath.join(self._dest_gsbase,
                       self._storage_playback_dirs.PlaybackGmExpectedDir()))

  def _Run(self):
    cmd = [self._PathToBinary('skdiff'),
           '--listfilenames',
           '--nodiffs',
           '--nomatch', 'TIMESTAMP',
           '--failonresult', 'DifferentPixels',
           '--failonresult', 'DifferentSizes',
           '--failonresult', 'DifferentOther',
           '--failonresult', 'Unknown',
           self._local_playback_dirs.PlaybackGmExpectedDir(),
           self._local_playback_dirs.PlaybackGmActualDir(),
           ]

    # Temporary list of builders who are allowed to fail this step without the
    # bot turning red.
    may_fail_with_warning = [
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Debug_32',
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_32',
        'Skia_Shuttle_Win7_Intel_Float_Debug_64',
        'Skia_Shuttle_Win7_Intel_Float_Release_64',
        'Skia_Mac_Float_Debug_64',
        'Skia_Mac_Float_Release_64',
        'Skia_MacMiniLion_Float_Debug_64',
        'Skia_MacMiniLion_Float_Release_64'
        ]

    if not self._gm_expected_exists_on_storage:
      # Copy images to expected directory if gm-expected has not been created in
      # Storage yet.
      print '\n\n=========Copying gm-actual to gm-expected locally=========\n\n'
      if os.path.exists(self._local_playback_dirs.PlaybackGmExpectedDir()):
        shutil.rmtree(self._local_playback_dirs.PlaybackGmExpectedDir())
      shutil.copytree(self._local_playback_dirs.PlaybackGmActualDir(),
                      self._local_playback_dirs.PlaybackGmExpectedDir())
      if self._do_upload_results:
        # Copy expected images to Google Storage since they do not exist yet.
        print '\n\n========Uploading gm-expected to Google Storage========\n\n'
        gs_utils.CopyStorageDirectory(
            src_dir=os.path.join(
                self._local_playback_dirs.PlaybackGmExpectedDir(), '*'),
            dest_dir=posixpath.join(
                self._dest_gsbase,
                self._storage_playback_dirs.PlaybackGmExpectedDir()),
            gs_acl=PLAYBACK_CANNED_ACL)
        # Add a TIMESTAMP file to the gm-expected directory in Google Storage so
        # we can use directory level rsync like functionality.
        print '\n\n=========Adding TIMESTAMP for gm-expected=========\n\n'
        gs_utils.WriteCurrentTimeStamp(
            gs_base=self._dest_gsbase,
            dest_dir=self._storage_playback_dirs.PlaybackGmExpectedDir(),
            local_dir=self._local_playback_dirs.PlaybackGmExpectedDir(),
            gs_acl=PLAYBACK_CANNED_ACL)

    elif not gs_utils.AreTimeStampsEqual(
        local_dir=self._local_playback_dirs.PlaybackGmExpectedDir(),
        gs_base=self._dest_gsbase,
        gs_relative_dir=self._storage_playback_dirs.PlaybackGmExpectedDir()):
      file_utils.CreateCleanLocalDir(
          self._local_playback_dirs.PlaybackGmExpectedDir())
      # Download expected images from Google Storage to the local directory.
      print '\n\n=======Downloading gm-expected from Google Storage=======\n\n'
      gs_utils.CopyStorageDirectory(
          src_dir=posixpath.join(
              self._dest_gsbase,
              self._storage_playback_dirs.PlaybackGmExpectedDir(),
              '*'),
          dest_dir=self._local_playback_dirs.PlaybackGmExpectedDir(),
          gs_acl=PLAYBACK_CANNED_ACL)

    else:
      print '\n\n=======Local gm-expected directory is current=======\n\n'

    # Debugging statements.
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

    try:
      print '\n\n=========Running GM Comparision=========\n\n'
      shell_utils.Bash(cmd)
    except Exception as e:
      print '\n\n=========GM Comparision Failed!=========\n\n'
      if self._do_upload_results:
        # Copy actual images to Google Storage only if the TIMESTAMPS are
        # different.
        if not gs_utils.AreTimeStampsEqual(
            local_dir=self._local_playback_dirs.PlaybackGmActualDir(),
            gs_base=self._dest_gsbase,
            gs_relative_dir=self._storage_playback_dirs.PlaybackGmActualDir()):
          print '\n\n=========Uploading gm-actual to Google Storage========\n\n'
          gs_utils.CopyStorageDirectory(
              src_dir=os.path.join(
                  self._local_playback_dirs.PlaybackGmActualDir(), '*'),
              dest_dir=posixpath.join(
                  self._dest_gsbase,
                  self._storage_playback_dirs.PlaybackGmActualDir()),
              gs_acl=PLAYBACK_CANNED_ACL)
          # Add a TIMESTAMP file to the gm-actual directory in Google Storage so
          # that rebaselining will be a simple directory copy from gm-actual to
          # gm-expected.
          print '\n\n=========Adding TIMESTAMP for gm-actual=========\n\n'
          gs_utils.WriteCurrentTimeStamp(
              gs_base=self._dest_gsbase,
              dest_dir=self._storage_playback_dirs.PlaybackGmActualDir(),
              local_dir=self._local_playback_dirs.PlaybackGmActualDir(),
              gs_acl=PLAYBACK_CANNED_ACL)
        else:
          print '\n\n=======Storage gm-actual directory is current=======\n\n'
      if self._builder_name in may_fail_with_warning:
        raise BuildStepWarning(e)
      else:
        raise


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CompareAndUploadWebpageGMs))
