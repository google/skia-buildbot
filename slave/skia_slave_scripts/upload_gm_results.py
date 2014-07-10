#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload actual GM results to the cloud to allow for rebaselining."""

from build_step import BuildStep
from utils import old_gs_utils as gs_utils
import os
import posixpath
import re
import shutil
import skia_vars
import sys
import tempfile


GS_SUMMARIES_BUCKET = 'gs://chromium-skia-gm-summaries'


class UploadGMResults(BuildStep):
  def __init__(self, timeout=4800, **kwargs):
    super(UploadGMResults, self).__init__(timeout=timeout, **kwargs)

  def _GSUploadAllImages(self, src_dir):
    """Upload all image files from src_dir to Google Storage.
    We know that GM wrote out these image files with a filename pattern we
    can use to generate the checksum-based Google Storage paths."""
    all_files = sorted(os.listdir(src_dir))
    def filematcher(filename):
      return filename.endswith('.png')
    files_to_upload = filter(filematcher, all_files)
    print 'Uploading %d GM-actual files to Google Storage...' % (
        len(files_to_upload))
    if not files_to_upload:
      return
    filename_pattern = re.compile('^([^_]+)_(.+)_([^_]+)\.png$')

    gm_actuals_subdir = 'gm'
    temp_root = tempfile.mkdtemp()
    try:
      # Copy all of the desired files to a staging dir, with new filenames.
      for filename in files_to_upload:
        match = filename_pattern.match(filename)
        if not match:
          print 'Warning: found no images matching pattern "%s"' % filename
          continue
        (hashtype, test, hashvalue) = match.groups()
        src_filepath = os.path.join(src_dir, filename)
        temp_dir = os.path.join(temp_root, gm_actuals_subdir, hashtype, test)
        if not os.path.isdir(temp_dir):
          os.makedirs(temp_dir)
        shutil.copy(src_filepath, os.path.join(temp_dir, hashvalue + '.png'))

      # Upload the entire staging dir to Google Storage.
      # At present, this will merge the entire contents of [temp_root]/gm
      # into the existing contents of gs://chromium-skia-gm/gm .
      #
      # TODO(epoger): Add a "noclobber" mode to gs_utils.upload_dir_contents()
      # and use it here so we don't re-upload image files we already have
      # in Google Storage.
      gs_utils.upload_dir_contents(
          local_src_dir=os.path.abspath(
              os.path.join(temp_root, gm_actuals_subdir)),
          remote_dest_dir=posixpath.join(
              skia_vars.GetGlobalVariable('googlestorage_bucket'),
              gm_actuals_subdir),
          gs_acl='public-read',
          http_header_lines=['Cache-Control:public,max-age=3600'])
    finally:
      shutil.rmtree(temp_root)

  def _GSUploadJsonFiles(self, src_dir, step_name=None):
    """Upload just the JSON files within src_dir to GS_SUMMARIES_BUCKET.

    Args:
      src_dir: (string) directory to upload contents of
      step_name: (string) name of the step that is performing this action;
          defaults to self.__class__.__name__
    """
    if not step_name:
      step_name = self.__class__.__name__
    all_files = sorted(os.listdir(src_dir))
    def filematcher(filename):
      return filename.endswith('.json')
    files_to_upload = filter(filematcher, all_files)
    print 'Uploading %d JSON files to Google Storage: %s...' % (
        len(files_to_upload), files_to_upload)
    gs_dest_dir = posixpath.join(
        GS_SUMMARIES_BUCKET, self._args['builder_name'])
    for filename in files_to_upload:
      src_path = os.path.join(src_dir, filename)
      gs_dest_path = posixpath.join(gs_dest_dir, filename)
      gs_utils.upload_file(
          local_src_path=src_path, remote_dest_path=gs_dest_path,
          gs_acl='public-read', only_if_modified=True,
          http_header_lines=['Cache-Control:public,max-age=3600'])

  def _Run(self):
    # TODO(epoger): Can't we get gm_output_dir from BuildStep._gm_actual_dir ?
    gm_output_dir = os.path.join(os.pardir, os.pardir, 'gm', 'actual',
                                 self._args['builder_name'])
    self._GSUploadAllImages(src_dir=gm_output_dir)
    self._GSUploadJsonFiles(src_dir=gm_output_dir)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadGMResults))
