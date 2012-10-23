#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload actual GM results to the skia-autogen SVN repository to aid in
rebaselining. """

from build_step import BuildStep
from utils import merge_into_svn
from utils import misc
import optparse
import os
import sys

# Class we can set attributes on, to emulate optparse-parsed options.
# TODO: Remove the need for this by passing parameters into MergeIntoSvn() some
# other way
class Options(object):
  pass

class UploadGMResults(BuildStep):
  def __init__(self, args, attempts=5):
    super(UploadGMResults, self).__init__(args, attempts)

  def _Run(self):
    # TODO these constants should actually be shared by multiple build steps
    gm_actual_basedir = os.path.join(os.pardir, os.pardir, 'gm', 'actual')
    gm_merge_basedir = os.path.join(os.pardir, os.pardir, 'gm', 'merge')
    autogen_svn_baseurl = 'https://skia-autogen.googlecode.com/svn'
    gm_actual_svn_baseurl = '%s/%s' % (autogen_svn_baseurl, 'gm-actual')
    autogen_svn_username_file = self._args['autogen_svn_username_file']
    autogen_svn_password_file = self._args['autogen_svn_password_file']
  
    # Call MergeIntoSvn to actually perform the work.
    # TODO: We should do something a bit more sophisticated, to address
    # https://code.google.com/p/skia/issues/detail?id=720 ('UploadGMs step
    # should be skipped when re-running old revisions of the buildbot')
    merge_options = Options()
    merge_options.commit_message = 'UploadGMResults of r%s on %s' % (
        self._revision, self._args['builder_name'])
    merge_options.dest_svn_url = '%s/%s/%s/%s' % (
        gm_actual_svn_baseurl, self._args['gm_image_subdir'],
        self._args['builder_name'], self._args['gm_image_subdir'])
    merge_options.merge_dir_path = os.path.join(gm_merge_basedir,
                                                self._args['gm_image_subdir'])
    merge_options.source_dir_path = os.path.join(gm_actual_basedir,
                                                 self._args['gm_image_subdir'])
    merge_options.svn_password_file = autogen_svn_password_file
    merge_options.svn_username_file = autogen_svn_username_file
    merge_into_svn.MergeIntoSvn(merge_options)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadGMResults))
