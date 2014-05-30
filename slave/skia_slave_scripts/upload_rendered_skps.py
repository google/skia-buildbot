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

GS_SUMMARIES_BUCKET = 'gs://chromium-skia-skp-summaries'
SUBDIR_NAME = 'rendered-skps'


class UploadRenderedSKPs(BuildStep):

  def __init__(self, attempts=3, **kwargs):
    super(UploadRenderedSKPs, self).__init__(
        attempts=attempts, **kwargs)

  def _Run(self):
    # Upload individual image files to Google Storage.
    #
    # TODO(epoger): Add a "noclobber" mode to gs_utils.upload_dir_contents()
    # and use it here so we don't re-upload image files we already have
    # in Google Storage.
    #
    # TODO(epoger): Change ACLs of files uploaded to Google Storage to
    # google.com:READ .  See _SetGoogleReadACLs() in
    # https://skia.googlesource.com/buildbot/+/master/slave/skia_slave_scripts/
    #         webpages_playback.py
    src_dir = os.path.abspath(self.playback_actual_images_dir)
    if os.listdir(src_dir):
      dest_dir = posixpath.join(
          skia_vars.GetGlobalVariable('googlestorage_bucket'), SUBDIR_NAME)
      print 'Uploading image files from %s to %s.' % (src_dir, dest_dir)
      gs_utils.upload_dir_contents(
          local_src_dir=src_dir, remote_dest_dir=dest_dir,
          gs_acl=PLAYBACK_CANNED_ACL)
    else:
      print ('No image files in %s, so skipping upload to Google Storage.' %
             src_dir)

    # Upload image summaries (checksums) to Google Storage.
    src_dir = os.path.abspath(self.playback_actual_summaries_dir)
    filenames = os.listdir(src_dir)
    if filenames:
      dest_dir = posixpath.join(GS_SUMMARIES_BUCKET, self._args['builder_name'])
      print ('Uploading %d image summaries from %s to %s: %s' % (
          len(filenames), src_dir, dest_dir, filenames))
      for filename in filenames:
        src_path = os.path.join(src_dir, filename)
        dest_path = posixpath.join(dest_dir, filename)
        gs_utils.upload_file(
            local_src_path=src_path, remote_dest_path=dest_path,
            gs_acl=PLAYBACK_CANNED_ACL, only_if_modified=True)
    else:
      print ('No image summaries in %s, so skipping upload to Google Storage.' %
             src_dir)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadRenderedSKPs))
