#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Uploads the results of render_skps.py."""

import os
import posixpath
import sys

from build_step import BuildStep, PLAYBACK_CANNED_ACL
from utils import gs_utils
import skia_vars
import upload_gm_results

SUBDIR_NAME = 'rendered-skps'


class UploadRenderedSKPs(upload_gm_results.UploadGMResults):

  def __init__(self, attempts=3, **kwargs):
    super(UploadRenderedSKPs, self).__init__(
        attempts=attempts, **kwargs)

  def _Run(self):
    gs_utils.copy_storage_directory(
        src_dir=os.path.abspath(self.playback_actual_images_dir),
        dest_dir=posixpath.join(
            skia_vars.GetGlobalVariable('googlestorage_bucket'), SUBDIR_NAME),
        gs_acl=PLAYBACK_CANNED_ACL)
    # TODO(epoger): Change ACLs of files uploaded to Google Storage to
    # google.com:READ .  See _SetGoogleReadACLs() in
    # https://skia.googlesource.com/buildbot/+/master/slave/skia_slave_scripts/webpages_playback.py
    self._SVNUploadJsonFiles(src_dir=self.playback_actual_summaries_dir,
                             dest_subdir=SUBDIR_NAME)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadRenderedSKPs))
