#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Uploads the results of render_skps.py."""

import os
import posixpath
import sys

from build_step import BuildStep
from utils import gs_utils
from utils import old_gs_utils
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
    gs = gs_utils.GSUtils()
    gs_bucket = gs.without_gs_prefix(
        skia_vars.GetGlobalVariable('googlestorage_bucket'))

    src_dir = os.path.abspath(self.playback_actual_images_dir)
    if os.listdir(src_dir):
      print 'Uploading image files from %s to bucket=%s, dir=%s' % (
          src_dir, gs_bucket, SUBDIR_NAME)
      gs.upload_dir_contents(
          source_dir=src_dir, dest_bucket=gs_bucket, dest_dir=SUBDIR_NAME,
          predefined_acl=gs.PLAYBACK_CANNED_ACL,
          fine_grained_acl_list=gs.PLAYBACK_FINEGRAINED_ACL_LIST)
    else:
      print ('Skipping upload to Google Storage, because no image files in %s' %
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
        # It's important to only upload the summary file when it has changed,
        # because we use the history of the file in Google Storage to tell us
        # when any of the results changed.
        #
        # TODO(epoger): Once gs_utils.upload_file() supports only_if_modified
        # parameter, start using it, so we can set fine_grained_acl_list like
        # we do above... we'll need that for google.com users to be able to
        # download the summary files.
        old_gs_utils.upload_file(
            local_src_path=src_path, remote_dest_path=dest_path,
            gs_acl=gs_utils.GSUtils.PLAYBACK_CANNED_ACL, only_if_modified=True)
    else:
      print ('Skipping upload to Google Storage, because no image summaries '
             'in %s' % src_dir)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadRenderedSKPs))
