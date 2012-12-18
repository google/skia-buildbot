#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload benchmark graphs. """

from build_step import BuildStep, BuildStepWarning
from utils import bench_common
from utils import misc
from utils import sync_bucket_subdir
from utils import upload_to_bucket

import optparse
import os
import sys


class UploadBenchGraphs(BuildStep):

  def __init__(self, args, attempts=5):
    super(UploadBenchGraphs, self).__init__(args, attempts)

  def _GetPerfGraphsDir(self):
    return self._perf_graphs_dir

  def _GetBucketSubdir(self):
    return None

  def _RunInternal(self, representation):
    graph_filepath = bench_common.GraphFilePath(self._GetPerfGraphsDir(),
                                                self._builder_name,
                                                representation)
    if os.path.exists(graph_filepath):
      dest_gsbase = (self._args.get('dest_gsbase') or
                     sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)
      upload_to_bucket.upload_to_bucket(source_filepath=graph_filepath,
                                        dest_gsbase=dest_gsbase,
                                        subdir=self._GetBucketSubdir())
    else:
      raise BuildStepWarning(
          '\n\n%s does not exist! Skipping graph upload!' % graph_filepath)

  def _Run(self):
    if self._is_try:
      raise BuildStepWarning('Not yet uploading results for try jobs.') # TODO

    for rep in ['avg', 'min', 'med', '25th']:
      self._RunInternal(rep)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadBenchGraphs))
