#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload actual GM results to the skia-autogen SVN repository to aid in
rebaselining. """

from build_step import BuildStep
from common import chromium_utils
from config_private import AUTOGEN_SVN_BASEURL
from utils import gs_utils, merge_into_svn
import os
import re
import shutil
import skia_vars
import sys
import tempfile


# Class we can set attributes on, to emulate optparse-parsed options.
# TODO: Remove the need for this by passing parameters into MergeIntoSvn() some
# other way
class Options(object):
  pass


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
      gs_utils.copy_storage_directory(
          src_dir=os.path.abspath(os.path.join(temp_root, gm_actuals_subdir)),
          dest_dir=skia_vars.GetGlobalVariable('googlestorage_bucket'),
          gs_acl='public-read',
          http_header_lines=['Cache-Control:public,max-age=3600'])
    finally:
      shutil.rmtree(temp_root)

  def _SVNUploadDir(self, src_dir):
    """Upload the entire contents of src_dir to the skia-autogen SVN repo."""

    # TODO these constants should actually be shared by multiple build steps
    gm_merge_basedir = os.path.join(os.pardir, os.pardir, 'gm', 'merge')
    gm_actual_svn_baseurl = '%s/%s' % (AUTOGEN_SVN_BASEURL, 'gm-actual')
    autogen_svn_username_file = self._args['autogen_svn_username_file']
    autogen_svn_password_file = self._args['autogen_svn_password_file']

    # Call MergeIntoSvn to actually perform the work.
    # TODO: We should do something a bit more sophisticated, to address
    # https://code.google.com/p/skia/issues/detail?id=720 ('UploadGMs step
    # should be skipped when re-running old revisions of the buildbot')
    merge_options = Options()
    # pylint: disable=W0201
    merge_options.commit_message = 'UploadGMResults of r%s on %s' % (
        self._got_revision, self._args['builder_name'])
    # pylint: disable=W0201
    merge_options.dest_svn_url = '%s/%s' % (
        gm_actual_svn_baseurl, self._args['builder_name'])
    # pylint: disable=W0201
    merge_options.merge_dir_path = os.path.join(gm_merge_basedir,
                                                self._args['builder_name'])
    # Clear out the merge_dir, in case it has old imagefiles in it from the
    # bad old days when we were still uploading actual images to skia-autogen.
    # This resolves https://code.google.com/p/skia/issues/detail?id=1362 ('some
    # buildbots are still uploading image files to skia-autogen after r9709')
    #
    # We wouldn't want to do this for a mergedir like that used for the
    # Doxygen docs, since that dir holds so many files.. but this mergedir
    # only holds the actual-results.json file now.  So the overhead of
    # downloading that file from the repo every time isn't a big deal.
    chromium_utils.RemoveDirectory(merge_options.merge_dir_path)
    # pylint: disable=W0201
    merge_options.source_dir_path = src_dir
    # pylint: disable=W0201
    merge_options.svn_password_file = autogen_svn_password_file
    # pylint: disable=W0201
    merge_options.svn_username_file = autogen_svn_username_file
    merge_into_svn.MergeIntoSvn(merge_options)

  def _SVNUploadJsonFiles(self, src_dir):
    """Upload just the JSON files within src_dir to the skia-autogen
    SVN repo."""
    tempdir = tempfile.mkdtemp()
    all_files = sorted(os.listdir(src_dir))
    def filematcher(filename):
      return filename.endswith('.json')
    files_to_upload = filter(filematcher, all_files)
    print 'Uploading %d JSON files to skia-autogen: %s...' % (
        len(files_to_upload), files_to_upload)
    for filename in files_to_upload:
      src_filepath = os.path.join(src_dir, filename)
      shutil.copy(src_filepath, tempdir)
    self._SVNUploadDir(src_dir=tempdir)
    shutil.rmtree(tempdir)

  def _Run(self):
    gm_output_dir = os.path.join(os.pardir, os.pardir, 'gm', 'actual',
                                 self._args['builder_name'])
    self._GSUploadAllImages(src_dir=gm_output_dir)
    self._SVNUploadJsonFiles(src_dir=gm_output_dir)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadGMResults))
