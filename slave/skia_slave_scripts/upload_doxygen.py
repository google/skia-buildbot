#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Upload Doxygen documentation."""

import os
import sys

from build_step import BuildStep
from config_private import AUTOGEN_SVN_BASEURL
from utils import merge_into_svn
import generate_doxygen


# Class we can set attributes on, to emulate optparse-parsed options.
# TODO(epoger): Delete this once we are uploading doxygen docs using some
# mechanism other than SVN.
class Options(object):
  pass


class UploadDoxygen(BuildStep):

  def _SVNUploadDir(self, src_dir, dest_subdir, step_name):
    """Upload the entire contents of src_dir to the skia-autogen SVN repo.

    This is mostly copy-pasted from _SVNUploadDir() in upload_gm_results.py.
    TODO(epoger): Delete this and use some mechanism other than SVN.

    Args:
      src_dir: (string) directory to upload contents of
      dest_subdir: (string) subdir on the skia-autogen repo to upload into
      step_name: (string) name of the step that is performing this action
    """
    merge_basedir = os.path.join(os.pardir, os.pardir, 'doxygen-merge')
    actual_svn_baseurl = '%s/%s' % (AUTOGEN_SVN_BASEURL, dest_subdir)
    autogen_svn_username_file = self._args['autogen_svn_username_file']
    autogen_svn_password_file = self._args['autogen_svn_password_file']

    merge_options = Options()
    # pylint: disable=W0201
    merge_options.commit_message = '%s of r%s on %s' % (
        step_name, self._got_revision, self._args['builder_name'])
    # pylint: disable=W0201
    merge_options.dest_svn_url = actual_svn_baseurl
    # pylint: disable=W0201
    merge_options.merge_dir_path = os.path.join(merge_basedir,
                                                self._args['builder_name'])
    # pylint: disable=W0201
    merge_options.source_dir_path = src_dir
    # pylint: disable=W0201
    merge_options.svn_password_file = autogen_svn_password_file
    # pylint: disable=W0201
    merge_options.svn_username_file = autogen_svn_username_file
    merge_into_svn.MergeIntoSvn(merge_options)

  def _Run(self):
    self._SVNUploadDir(
        src_dir=generate_doxygen.DOXYGEN_WORKING_DIR,
        dest_subdir='docs',
        step_name=self.__class__.__name__)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadDoxygen))
