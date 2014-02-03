#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Download the image files needed to run skimage tool. """

from build_step import BuildStep
from utils import gs_utils
from utils import sync_bucket_subdir
import posixpath
import sys

class DownloadSKImageFiles(BuildStep):
  def __init__(self, timeout=12800, no_output_timeout=9600, **kwargs):
    super (DownloadSKImageFiles, self).__init__(
        timeout=timeout,
        no_output_timeout=no_output_timeout,
        **kwargs)

  def _DownloadSKImagesFromStorage(self):
    """Copies over image files from Google Storage if the timestamps differ."""
    dest_gsbase = (self._args.get('dest_gsbase') or
                   sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)
    print '\n\n========Downloading image files from Google Storage========\n\n'
    gs_relative_dir = posixpath.join('skimage', 'input')
    gs_utils.download_directory_contents_if_changed(
        gs_base=dest_gsbase,
        gs_relative_dir=gs_relative_dir,
        local_dir=self._skimage_in_dir)

  def _Run(self):
    # Locally copy image files from GoogleStorage.
    self._DownloadSKImagesFromStorage()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(DownloadSKImageFiles))
