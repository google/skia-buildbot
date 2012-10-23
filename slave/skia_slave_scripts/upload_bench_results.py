#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload benchmark performance data results. """

from build_step import BuildStep
from utils import bench_common
from utils import misc
from utils import sync_bucket_subdir
import optparse
import os
import posixpath
import sys

class UploadBenchResults(BuildStep):
  def __init__(self, args, attempts=5):
    super(UploadBenchResults, self).__init__(args, attempts)

  def _RunInternal(self):
    return sync_bucket_subdir.SyncBucketSubdir(dir=self._perf_data_dir,
               subdir=posixpath.join('perfdata', self._builder_name),
               do_upload=True,
               do_download=False)

  def _Run(self):
    self._RunInternal()

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadBenchResults))
