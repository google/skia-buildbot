#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload bench expectation file to the skia-autogen SVN repository. """

from build_step import BuildStep
from common import chromium_utils
from config_private import AUTOGEN_SVN_BASEURL
from utils import merge_into_svn
import os
import shutil
import sys
import tempfile


# Class we can set attributes on, to emulate optparse-parsed options.
# See upload_gm_results.py for details.
class Options(object):
  pass


class UploadBenchExpectations(BuildStep):
  def __init__(self, timeout=300, **kwargs):
    super(UploadBenchExpectations, self).__init__(timeout=timeout, **kwargs)

  def _SVNUploadDir(self, src_dir):
    """Upload the entire contents of src_dir to the skia-autogen SVN repo."""

    # TODO these constants should actually be shared by multiple build steps
    bench_merge_basedir = os.path.join(os.pardir, os.pardir, 'bench', 'merge')
    bench_actual_svn_baseurl = '%s/%s' % (AUTOGEN_SVN_BASEURL, 'bench')
    autogen_svn_username_file = self._args['autogen_svn_username_file']
    autogen_svn_password_file = self._args['autogen_svn_password_file']

    # Call MergeIntoSvn to actually perform the work.
    merge_options = Options()
    # pylint: disable=W0201
    merge_options.commit_message = 'UploadBenchExpectations of %s on %s.' % (
        self._got_revision, self._args['builder_name'])
    # pylint: disable=W0201
    merge_options.dest_svn_url = bench_actual_svn_baseurl
    # pylint: disable=W0201
    merge_options.merge_dir_path = bench_merge_basedir

    chromium_utils.RemoveDirectory(merge_options.merge_dir_path)
    # pylint: disable=W0201
    merge_options.source_dir_path = src_dir
    # pylint: disable=W0201
    merge_options.svn_password_file = autogen_svn_password_file
    # pylint: disable=W0201
    merge_options.svn_username_file = autogen_svn_username_file
    merge_into_svn.MergeIntoSvn(merge_options)

  def _Run(self):
    self._SVNUploadDir(src_dir=self._perf_autogen_upload_dir)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadBenchExpectations))
