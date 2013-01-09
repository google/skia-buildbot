#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Rebaselines the specified gm_image_subdir or all gm_image_subdirs.

Note: Please make sure the gm-actuals of the builders of interest are not being
updated at the time this script is run.

Usage:
cd ../buildbot/slave/skia_slave_scripts
python rebaseline_webpage_images.py base-android-xoom
OR
python rebaseline_webpage_images.py base-android-xoom,base-android-nexus-s
OR
python rebaseline_webpage_images.py all
"""

import os
import posixpath
import sys
import time

# Set the PYTHONPATH for this script to include chromium_buildbot scripts and
# site_config and skia_slave_scripts.
sys.path.append(
    os.path.join(os.pardir, 'third_party', 'chromium_buildbot', 'scripts'))
sys.path.append(
    os.path.join(os.pardir, 'third_party', 'chromium_buildbot', 'site_config'))
sys.path.append(os.path.join(os.pardir, 'slave', 'skia_slave_scripts'))

from build_step import PLAYBACK_CANNED_ACL
from common import chromium_utils
from slave import slave_utils
from slave import svn
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


commit_whitespace_change = len(sys.argv) == 5
if len(sys.argv) != 2 and not commit_whitespace_change:
  print '\n\nUsage: python %s base-android-nexus-10\n' % (
      os.path.basename(sys.argv[0]))
  print 'Or to rebaseline a list of platforms run-'
  print 'Usage: python %s base-android-nexus-10,base-android-nexus-s\n' % (
      os.path.basename(sys.argv[0]))  
  print 'Or to rebaseline ALL platforms (very long running time) run-'
  print 'Usage: python %s %s\n' % (os.path.basename(sys.argv[0]),
                                   ARG_FOR_ALL_IMAGES)
  print ('You can also commit a whitespace change automatically when the '
         'rebaseline is completed, by specifying the trunk location, '
         'svn_username and svn_password.')
  print ('Usage: python %s base-android-nexus-10 skia-trunk-dir '
         'svn-username svn-password\n\n' % os.path.basename(sys.argv[0]))
  sys.exit(1)


gm_image_subdir = sys.argv[1]
gm_images_seq = []
if gm_image_subdir == ARG_FOR_ALL_IMAGES:
  gm_images_seq = GM_IMAGE_TO_BASELINE_BUILDER.keys()
elif ',' in gm_image_subdir:
  gm_images_seq.extend(gm_image_subdir.split(','))
else:
  gm_images_seq.append(gm_image_subdir)

# Check that all specified gm_image_subdirs are valid
for gm_image_subdir in gm_images_seq:
  if not gm_image_subdir in GM_IMAGE_TO_BASELINE_BUILDER:
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
  print '\n\n'
  print '======================================================================'
  print 'Rebaselining %s images' % gm_image_subdir
  print '======================================================================'

  builder_name = GM_IMAGE_TO_BASELINE_BUILDER[gm_image_subdir]
  
  storage_playback_dirs = playback_dirs.StorageSkpPlaybackDirs(
    builder_name=builder_name,
    gm_image_subdir=gm_image_subdir,
    perf_output_basedir=None)

  gm_actual_dir = posixpath.join(
      dest_gsbase, storage_playback_dirs.PlaybackGmActualDir())
  gm_expected_dir = posixpath.join(
      dest_gsbase, storage_playback_dirs.PlaybackGmExpectedDir())

  print '\n\n=======Throw an Exception if gm_actual_dir does not exist======='
  if not gs_utils.DoesStorageObjectExist(gm_actual_dir):
    raise Exception("%s does not exist in Google Storage!" % gm_actual_dir)

  print '\n\n=======Delete contents of gm_expected_dir======='
  gs_utils.DeleteStorageObject(gm_expected_dir)

  print '\n\n=====Copy all contents from gm_actual_dir to gm_expected_dir======'

  # Gather list of all files
  gm_actual_contents = gs_utils.ListStorageDirectory(
      dest_gsbase, storage_playback_dirs.PlaybackGmActualDir())

  # Remove TIMESTAMP_* and COMPARISON files from the list
  for file_to_remove in (
      gs_utils.TIMESTAMP_STARTED_FILENAME,
      gs_utils.TIMESTAMP_COMPLETED_FILENAME,
      compare_and_upload_webpage_gms.LAST_COMPARISON_FILENAME):
    gs_file_to_remove = posixpath.join(gm_actual_dir, file_to_remove)
    if gs_file_to_remove in gm_actual_contents:
      gm_actual_contents.remove(gs_file_to_remove)

  # Copy over files in chunks.
  for files_chunk in gs_utils._GetChunks(gm_actual_contents,
                                         gs_utils.FILES_CHUNK):
    gsutil = slave_utils.GSUtilSetup()
    command = ([gsutil, 'cp'] + files_chunk +
               [posixpath.join(gm_expected_dir, '')])
    if chromium_utils.RunCommand(command) != 0:
      raise Exception(
          'Could not upload the chunk to Google Storage! The chunk: %s'
          % files_chunk)

  print '\n\n=======Update gm_expected_dir timestamp======='
  gs_utils.WriteTimeStampFile(
      timestamp_file_name=gs_utils.TIMESTAMP_COMPLETED_FILENAME,
      timestamp_value=time.time(),
      gs_base=dest_gsbase,
      gs_relative_dir=storage_playback_dirs.PlaybackGmExpectedDir(),
      gs_acl=PLAYBACK_CANNED_ACL,
      local_dir=None)


# Submit whitespace change to trigger rebuilds if skia trunk location, svn
# username and password have been provided.
if commit_whitespace_change:
  skia_trunk_dir = sys.argv[2]
  svn_username = sys.argv[3]
  svn_password = sys.argv[4]
  repo = svn.Svn(skia_trunk_dir, svn_username, svn_password,
                 additional_svn_flags=['--trust-server-cert', '--no-auth-cache',
                                       '--non-interactive'])
  whitespace_file = open(os.path.join(skia_trunk_dir, 'whitespace.txt'), 'a')
  try:
    whitespace_file.write('\n')
  finally:
    whitespace_file.close()
  print '\n\n=======Submit whitespace change to trigger rebuilds======='
  repo._RunSvnCommand(
      ['commit', '--message',
       'Rebaselined webpage image GMs for %s on Google Storage.'
           % gm_images_seq,
       'whitespace.txt'])
