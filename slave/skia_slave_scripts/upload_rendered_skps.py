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
    # Upload individual image files to Google Storage.
    src_dir = os.path.abspath(self.playback_actual_images_dir)
    if os.listdir(src_dir):
      dest_dir = posixpath.join(
          skia_vars.GetGlobalVariable('googlestorage_bucket'), SUBDIR_NAME)
      print 'Uploading image files from %s to %s.' % (
          src_dir, dest_dir)
      gs_utils.upload_dir_contents(
          local_src_dir=src_dir, remote_dest_dir=dest_dir,
          gs_acl=PLAYBACK_CANNED_ACL)
    else:
      print ('No image files in %s, so skipping upload to Google Storage.' %
             src_dir)

    # Upload image summaries (checksums) to skia-autogen.
    #
    # TODO(epoger): Change ACLs of files uploaded to Google Storage to
    # google.com:READ .  See _SetGoogleReadACLs() in
    # https://skia.googlesource.com/buildbot/+/master/slave/skia_slave_scripts/webpages_playback.py
    self._SVNUploadJsonFiles(src_dir=self.playback_actual_summaries_dir,
                             dest_subdir=SUBDIR_NAME)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadRenderedSKPs))
