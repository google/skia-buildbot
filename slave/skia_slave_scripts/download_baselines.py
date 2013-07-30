#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Download the picture GM baselines. """

from build_step import BuildStep
from utils import gs_utils
from utils import sync_bucket_subdir
import posixpath
import sys


class DownloadBaselines(BuildStep):
  def __init__(self, timeout=6400, no_output_timeout=4800, **kwargs):
    super(DownloadBaselines, self).__init__(timeout=timeout,
                                            no_output_timeout=no_output_timeout,
                                            **kwargs)

  def _Run(self):
    # Skip this step for now until we have checksums.
    # Bug: https://code.google.com/p/skia/issues/detail?id=1455
    return

    dest_gsbase = (self._args.get('dest_gsbase') or
                   sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)

    gm_expected_exists_on_storage = gs_utils.DoesStorageObjectExist(
        posixpath.join(dest_gsbase,
                       self._storage_playback_dirs.PlaybackGmExpectedDir(),
                       gs_utils.TIMESTAMP_COMPLETED_FILENAME))

    if gm_expected_exists_on_storage:
      print '\n\n=======Downloading gm-expected from Google Storage=======\n\n'
      gs_utils.DownloadDirectoryContentsIfChanged(
          gs_base=dest_gsbase,
          gs_relative_dir=self._storage_playback_dirs.PlaybackGmExpectedDir(),
          local_dir=self._local_playback_dirs.PlaybackGmExpectedDir())


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(DownloadBaselines))