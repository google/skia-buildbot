#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Rebaselines the specified gm_image_subdir or all gm_image_subdirs.

Usage:

cd ../buildbot/slave/skia_slave_scripts
python rebaseline_webpage_images.py base-android-xoom
"""

import os
import posixpath
import sys
import time

# Set the PYTHONPATH for this script to include chromium_buildbot scripts and
# site_config.
sys.path.append(
    os.path.join(os.pardir, os.pardir, 'third_party', 'chromium_buildbot',
                 'scripts'))
sys.path.append(
    os.path.join(os.pardir, os.pardir, 'third_party', 'chromium_buildbot',
                 'site_config'))

from build_step import PLAYBACK_CANNED_ACL
from slave import slave_utils
from utils import gs_utils
from utils import sync_bucket_subdir

import compare_and_upload_webpage_gms
import playback_dirs


GM_IMAGE_TO_BASELINE_BUILDER = {
    'base-shuttle-win7-intel-float':
        'Skia_Shuttle_Win7_Intel_Float_Release_32',
    'base-shuttle-win7-intel-angle':
        'Skia_Shuttle_Win7_Intel_Float_ANGLE_Release_32',
    'base-shuttle-win7-intel-directwrite':
        'Skia_Shuttle_Win7_Intel_Float_DirectWrite_Release_32',
    'base-shuttle_ubuntu12_ati5770':
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64',
    'base-macmini':
        'Skia_Mac_Float_Release_32',
    'base-macmini-lion-float':
        'Skia_MacMiniLion_Float_Release_32',
    'base-android-galaxy-nexus':
        'Skia_GalaxyNexus_4-1_Float_Release_32',
    'base-android-nexus-7':
        'Skia_Nexus7_4-1_Float_Release_32',
    'base-android-nexus-s':
        'Skia_NexusS_4-1_Float_Release_32',
    'base-android-xoom':
        'Skia_Xoom_4-1_Float_Release_32',
    'base-android-nexus-10':
        'Skia_Nexus10_4-1_Float_Release_32'
}

ARG_FOR_ALL_IMAGES = 'all'


if len(sys.argv) != 2:
  print '\n\nUsage: python %s base-android-nexus-10\n' % (
      os.path.basename(sys.argv[0]))
  print 'Or to rebaseline ALL platforms (very long running time) run-'
  print 'Usage: python %s %s\n\n' % (os.path.basename(sys.argv[0]),
                                     ARG_FOR_ALL_IMAGES)
  sys.exit(1)


gm_image_subdir = sys.argv[1]
gm_images_seq = []
if gm_image_subdir == ARG_FOR_ALL_IMAGES:
  gm_images_seq = GM_IMAGE_TO_BASELINE_BUILDER.keys()
elif gm_image_subdir in GM_IMAGE_TO_BASELINE_BUILDER:
  gm_images_seq.append(gm_image_subdir)
else:
  raise ValueError(
      'Unknown specified gm_image_subdir (%s). Please use one from:\n%s' % (
          gm_image_subdir, str(GM_IMAGE_TO_BASELINE_BUILDER.keys())))


dest_gsbase = sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE


# Ensure the right .boto file is used by gsutil.
if not gs_utils.DoesStorageObjectExist(dest_gsbase):
  raise Exception(
      'Missing .boto file or .boto does not have the right credentials. Please '
      'see https://docs.google.com/a/google.com/document/d/1ZzHP6M5qACA9nJnLq'
      'OZr2Hl0rjYqE4yQsQWAfVjKCzs/edit (may have to request access)')


for gm_image_subdir in gm_images_seq:
  builder_name = GM_IMAGE_TO_BASELINE_BUILDER[gm_image_subdir]
  
  storage_playback_dirs = playback_dirs.StorageSkpPlaybackDirs(
    builder_name=builder_name,
    gm_image_subdir=gm_image_subdir,
    perf_output_basedir=None)

  gm_actual_dir = posixpath.join(
      dest_gsbase, storage_playback_dirs.PlaybackGmActualDir())
  gm_expected_dir = posixpath.join(
      dest_gsbase, storage_playback_dirs.PlaybackGmExpectedDir())
  gm_tmp_dir = posixpath.join(
      dest_gsbase, 'test', gm_image_subdir, builder_name)

  print '\n\nThrow an Exception if gm_actual_dir does not exist'
  if not gs_utils.DoesStorageObjectExist(gm_actual_dir):
    raise Exception("%s does not exist in Google Storage!" % gm_actual_dir)

  print '\n\nDelete contents of gm_expected_dir and gm_tmp_dir'
  gs_utils.DeleteStorageObject(gm_expected_dir)
  gs_utils.DeleteStorageObject(gm_tmp_dir)

  print '\n\nCopy all contents from gm_actual_dir to gm_expected_dir'
  # This is a 3 step process.
  # 1. Copy from gm_actual_dir to a temporary location.
  gs_utils.CopyStorageDirectory(
      src_dir=gm_actual_dir,
      dest_dir=gm_tmp_dir,
      gs_acl=PLAYBACK_CANNED_ACL)

  # 2. Delete TIMESTAMP_* and COMPARISON files from the temporary location.
  for file_to_remove in (
      gs_utils.TIMESTAMP_STARTED_FILENAME,
      gs_utils.TIMESTAMP_COMPLETED_FILENAME,
      compare_and_upload_webpage_gms.LAST_COMPARISON_FILENAME):
    gs_file_to_remove = posixpath.join(gm_tmp_dir, file_to_remove)
    gs_utils.DeleteStorageObject(gs_file_to_remove)

  # 3. Move files from the temporary location to gm_expected_dir.
  gs_utils.MoveStorageDirectory(
      src_dir=gm_tmp_dir,
      dest_dir=gm_expected_dir)

  print '\n\nUpdate gm_expected_dir timestamp'
  gs_utils.WriteTimeStampFile(
      timestamp_file_name=gs_utils.TIMESTAMP_COMPLETED_FILENAME,
      timestamp_value=time.time(),
      gs_base=dest_gsbase,
      gs_relative_dir=storage_playback_dirs.PlaybackGmExpectedDir(),
      gs_acl=PLAYBACK_CANNED_ACL,
      local_dir=None)
