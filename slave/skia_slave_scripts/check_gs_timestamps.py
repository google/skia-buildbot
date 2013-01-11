#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Checks that the timestamps of gm-actual directories on Google Storage match.

Checks if the TIMESTAMP_LAST_UPLOAD_STARTED and TIMESTAMP_LAST_UPLOAD_COMPLETED
files of all gm-actual directories on Google Storage match. This module can be
run from the command-line like this:

cd buildbot/third_party/chromium_buildbot/slave/\
Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64/build/trunk

PYTHONPATH=../../../../scripts:\
../../../../site_config \
python ../../../../../../slave/skia_slave_scripts/check_gs_timestamps.py \
--configuration "Debug" --target_platform "" --revision 0 \
--autogen_svn_baseurl "" --make_flags "" --test_args "" --gm_args "" \
--bench_args "" --num_cores 8 --perf_output_basedir "" \
--builder_name Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64 \
--got_revision 0 --gm_image_subdir base-shuttle_ubuntu12_ati5770 \
--is_try False --do_upload_results True --dest_gsbase gs://rmistry

"""

import posixpath
import sys

from utils import gs_utils
from utils import sync_bucket_subdir

import build_step


class CheckGoogleStorageTimestamps(build_step.BuildStep):

  def _Run(self):
    dest_gsbase = (self._args.get('dest_gsbase') or
                   sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)

    # gm-actual directories are of the form:
    # gs://chromium-skia-gm/playback/gm-actual/platform/builder-name/platform/*
    # We first get all directories under the first platform dir then get all
    # directories under the builder-name and finally get the timestamp files
    # from the final platform dir.

    # Get a list of all platform_dirs.
    platform_dirs = gs_utils.ListStorageDirectory(
        dest_gsbase=dest_gsbase,
        subdir=posixpath.join(
            self._storage_playback_dirs.PlaybackRootDir(), 'gm-actual'))

    # Get a list of all platform_builder_dirs.
    platform_builder_dirs = []
    for platform_dir in platform_dirs:
      platform_builder_dirs.extend(gs_utils.ListStorageDirectory(
          dest_gsbase=platform_dir,
          subdir=''))
    # TODO(rmistry): Ignoring Trybot builders for now. Enable them when they
    # are more stable. When I ran this script locally they were the only
    # builders which had differing started and completed timestamps.
    platform_builder_dirs = filter(
        lambda x: not x.endswith(posixpath.join('_Trybot', '')),
        platform_builder_dirs)

    # Get the final list of all platform_builder_platform_dirs.
    platform_builder_platform_dirs = []
    for platform_and_builder_dir in platform_builder_dirs:
      platform_builder_platform_dirs.extend(gs_utils.ListStorageDirectory(
          dest_gsbase=platform_and_builder_dir,
          subdir=''))

    # Check TIMESTAMP_LAST_UPLOAD_STARTED and TIMESTAMP_LAST_UPLOAD_COMPLETED in
    # each platform_builder_platform directory.
    failed_gm_actual_dirs = []
    for timestamp_dir in platform_builder_platform_dirs:
      gm_actual_started_timestamp = gs_utils.ReadTimeStampFile(
          timestamp_file_name=gs_utils.TIMESTAMP_STARTED_FILENAME,
          gs_base=timestamp_dir,
          gs_relative_dir='')
      gm_actual_completed_timestamp = gs_utils.ReadTimeStampFile(
          timestamp_file_name=gs_utils.TIMESTAMP_COMPLETED_FILENAME,
          gs_base=timestamp_dir,
          gs_relative_dir='')
      if gm_actual_started_timestamp != gm_actual_completed_timestamp:
        failed_gm_actual_dirs.append(timestamp_dir)

    if failed_gm_actual_dirs:
      exception_txt = (
          'These are the gm-actual directories with timestamps that do not '
          'match: %s\n'
          'This indicates one of two things (can be determined by examining '
          'the directory):\n'
          '* The builder is currently running and is in the process of '
          'updating its gm-actual directory. In this case we do not need to do '
          'anything.\n'
          '* The builder\'s gm-actual directory is in an inconsistent state '
          'and needs to be manually fixed by deleting its '
          'TIMESTAMP_LAST_UPLOAD_COMPLETED directory.\n\n'
              % failed_gm_actual_dirs)
      raise Exception(exception_txt)


if '__main__' == __name__:
  sys.exit(build_step.BuildStep.RunBuildStep(CheckGoogleStorageTimestamps))
