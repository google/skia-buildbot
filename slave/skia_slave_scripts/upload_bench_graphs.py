#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Upload benchmark graphs. """

from build_step import BuildStep
from utils import bench_common
from utils import misc
from utils import upload_to_bucket
import optparse
import os
import sys

class UploadBenchGraphs(BuildStep):
  def __init__(self, args, attempts=5):
    super(UploadBenchGraphs, self).__init__(args, attempts)

  def _RunInternal(self, representation):
    graph_filepath = bench_common.GraphFilePath(self._perf_graphs_dir,
                                                self._builder_name,
                                                representation)
    upload_to_bucket.upload_to_bucket(source_filepath=graph_filepath,
                                      dest_gsbase='gs://chromium-skia-gm')

  def _Run(self, args):
    for rep in ['avg', 'min', 'med']:
      self._RunInternal(rep)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(UploadBenchGraphs))
