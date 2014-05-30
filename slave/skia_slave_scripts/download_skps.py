#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Download the SKPs. """

from build_step import BuildStep
from utils import gs_utils
from utils import sync_bucket_subdir
import os
import posixpath
import sys


class DownloadSKPs(BuildStep):
  def __init__(self, timeout=12800, no_output_timeout=9600, **kwargs):
    super (DownloadSKPs, self).__init__(
        timeout=timeout,
        no_output_timeout=no_output_timeout,
        **kwargs)

  def _Run(self):
    dest_gsbase = (self._args.get('dest_gsbase') or
                   sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)
    version_file = 'SKP_VERSION'
    expected_skp_version = None
    with open(version_file) as f:
      expected_skp_version = f.read().rstrip()
    actual_skp_version = None
    actual_version_file = os.path.join(
        self._local_playback_dirs.PlaybackSkpDir(), version_file)
    if os.path.isfile(actual_version_file):
      with open(actual_version_file) as f:
        actual_skp_version = f.read().rstrip()
    if actual_skp_version != expected_skp_version:
      self._flavor_utils.CreateCleanHostDirectory(
          self._local_playback_dirs.PlaybackSkpDir())
      gs_relative_dir = (
          self._storage_playback_dirs.PlaybackSkpDir(expected_skp_version))
      print '\n\n========Downloading skp files from Google Storage========\n\n'
      gs_utils.download_dir_contents(
          remote_src_dir=posixpath.join(dest_gsbase, gs_relative_dir),
          local_dest_dir=self._local_playback_dirs.PlaybackRootDir())
      with open(actual_version_file, 'w') as f:
        f.write(expected_skp_version)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(DownloadSKPs))
