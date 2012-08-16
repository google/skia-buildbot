#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Generate performance graphs from bench output. """

from utils import bench_common
from utils import misc
from build_step import BuildStep
import errno
import os
import sys

class GenerateBenchGraphs(BuildStep):
  def _RunInternal(self, representation):
    try:
      os.makedirs(self._perf_graphs_dir)
    except OSError as e:
      if e.errno == errno.EEXIST:
        pass
      else:
        raise e
    path_to_bench_graph_svg = os.path.join('bench', 'bench_graph_svg.py')
    graph_title = 'Bench_Performance_for_%s' % self._builder_name
    graph_filepath = bench_common.GraphFilePath(self._perf_graphs_dir,
                                                self._builder_name,
                                                representation)
    cmd = ['python', path_to_bench_graph_svg,
           '-d', self._perf_data_dir,
           '-r', '-%d' % bench_common.BENCH_GRAPH_NUM_REVISIONS,
           '-f', '-%d' % bench_common.BENCH_GRAPH_NUM_REVISIONS,
           '-x', '%d' % bench_common.BENCH_GRAPH_X,
           '-y', '%d' % bench_common.BENCH_GRAPH_Y,
           '-l', graph_title,
           '-m', representation,
           '-o', graph_filepath,
           ]
    misc.Bash(cmd)

  def _Run(self, args):
    for rep in ['avg', 'min', 'med', '25th']:
      self._RunInternal(rep)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(GenerateBenchGraphs))
