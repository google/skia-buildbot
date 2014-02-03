#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run this before running any tests. """

from build_step import BuildStep
from utils import shell_utils
from utils import upload_to_bucket
import os
import posixpath
import skia_vars
import sys


GS_DRT_SUBDIR = 'chrome_drt_results'


class ChromeDRTCanaryUploadResults(BuildStep):
  def _Run(self):
    # Tar up the results.
    result_tarball = '%s_%s.tgz' % (self._builder_name,
                                    self._got_revision)
    shell_utils.run(['tar', '-cvzf', os.path.join(os.pardir, result_tarball),
                     self._flavor_utils.result_dir])

    # Upload to Google Storage
    upload_to_bucket.upload_to_bucket(
        os.path.join(os.pardir, result_tarball),
        skia_vars.GetGlobalVariable('googlestorage_bucket'),
        subdir=GS_DRT_SUBDIR)

    print 'To download the tarball, run this command:'
    gs_url = posixpath.join(
        skia_vars.GetGlobalVariable('googlestorage_bucket'),
        GS_DRT_SUBDIR,
        result_tarball)
    print 'gsutil cp %s <local_dir>' % gs_url


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeDRTCanaryUploadResults))
