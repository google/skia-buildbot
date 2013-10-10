#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Upload results from running skimage."""

import build_step
import os
import posixpath
# Must be imported after build_step, which adds site_config to the python path.
import skia_vars
import sys

from utils import gs_utils
from utils import sync_bucket_subdir
from build_step import PLAYBACK_CANNED_ACL
from build_step import BuildStep

SKP_TIMEOUT_MULTIPLIER = 8

class UploadSKImageResults(BuildStep):

  def __init__(
      self,
      timeout=build_step.DEFAULT_TIMEOUT * SKP_TIMEOUT_MULTIPLIER,
      no_output_timeout=(
          build_step.DEFAULT_NO_OUTPUT_TIMEOUT * SKP_TIMEOUT_MULTIPLIER),
      **kwargs):
    """Constructs an UploadSKImageResults BuildStep instance.

    timeout: maximum time allowed for this BuildStep. The default value here is
             increased because there could be a lot of images
             to be copied over to Google Storage.
    no_output_timeout: maximum time allowed for this BuildStep to run without
        any output.
    """
    build_step.BuildStep.__init__(self, timeout=timeout,
                                  no_output_timeout=no_output_timeout,
                                  **kwargs)

    self._dest_gsbase = (self._args.get('dest_gsbase') or
                         sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)

  def _Run(self):
    if self._do_upload_results:
      # Copy actual-results.json to skimage/output/actuals
      print '\n\n====Uploading skimage actual-results to Google Storage====\n\n'
      src_dir = os.path.abspath(os.path.join(self._skimage_out_dir,
                                             self._builder_name))
      dest_dir = posixpath.join(
          skia_vars.GetGlobalVariable('googlestorage_bucket'),
          'skimage', 'actuals')
      http_header_lines = ['Cache-Control:public,max-age=3600']
      gs_utils.CopyStorageDirectory(src_dir=src_dir,
                                    dest_dir=dest_dir,
                                    gs_acl='public-read',
                                    http_header_lines=http_header_lines)

      # Copy actual images and expectations to Google Storage.
      # TODO(scroggo): This step uploads both the expectations and the
      # mismatches, even though the expectations files were already uploaded
      # in the above step. Once skimage is rewritten to include the hash digest
      # in the filename, rewrite this to upload a folder (named <input image>)
      # containing the result (named <hash digest>.png), much like
      # upload_gm_results.
      print '\n\n========Uploading skimage results to Google Storage=======\n\n'
      relative_dir = posixpath.join('skimage', 'output', self._builder_name)
      gs_utils.UploadDirectoryContentsIfChanged(
          gs_base=self._dest_gsbase,
          gs_relative_dir=relative_dir,
          gs_acl=PLAYBACK_CANNED_ACL,
          force_upload=True,
          local_dir=self._skimage_out_dir)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadSKImageResults))
