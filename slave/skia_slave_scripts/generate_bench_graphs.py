#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Generate performance graphs from bench output. """

from build_step import BuildStep
from utils import bench_common
from utils import shell_utils
from utils import sync_bucket_subdir

import errno
import os
import posixpath
import sys


class GenerateBenchGraphs(BuildStep):
  # TODO(borenet): Temporarily increasing the timeout since the amount of data
  # dramatically increased with r7620.
  def __init__(self, timeout=9600, no_output_timeout=9600, **kwargs):
    super(GenerateBenchGraphs, self).__init__(
        timeout=timeout,
        no_output_timeout=no_output_timeout,
        **kwargs)

  def _GetPerfGraphsDir(self):
    return self._perf_graphs_dir

  def _GetPerfDataDir(self):
    return self._perf_data_dir

  def _GetBucketSubdir(self):
    return posixpath.join('perfdata', self._builder_name)

  def _RunInternal(self, representation):
    try:
      os.makedirs(self._GetPerfGraphsDir())
    except OSError as e:
      if e.errno == errno.EEXIST:
        pass
      else:
        raise e
    dest_gsbase = (self._args.get('dest_gsbase') or
                   sync_bucket_subdir.DEFAULT_PERFDATA_GS_BASE)

    path_to_bench_graph_svg = os.path.join('bench', 'bench_graph_svg.py')
    path_to_bench_expectations = os.path.join('bench',
                                              'bench_expectations.txt')
    graph_title = 'Bench_Performance_for_%s' % self._builder_name
    graph_filepath = bench_common.GraphFilePath(self._GetPerfGraphsDir(),
                                                self._builder_name,
                                                representation)
    sync_bucket_subdir.SyncBucketSubdir(directory=self._GetPerfDataDir(),
        dest_gsbase=dest_gsbase,
        subdir=self._GetBucketSubdir(),
        do_upload=False,
        do_download=True,
        min_download_revision=self._got_revision -
            bench_common.BENCH_GRAPH_NUM_REVISIONS)

    cmd = ['python', path_to_bench_graph_svg,
           '-d', self._GetPerfDataDir(),
           '-e', path_to_bench_expectations,
           '-r', '-%d' % bench_common.BENCH_GRAPH_NUM_REVISIONS,
           '-f', '-%d' % bench_common.BENCH_GRAPH_NUM_REVISIONS,
           '-x', '%d' % bench_common.BENCH_GRAPH_X,
           '-y', '%d' % bench_common.BENCH_GRAPH_Y,
           '-l', graph_title,
           '-m', representation,
           '-o', graph_filepath,
           ]
    if self._builder_name.find('_Win') >= 0:
      cmd.extend(['-i', 'c'])  # Ignore cpu time for Windows.

    shell_utils.Bash(cmd)

  def _Run(self):
    for rep in ['avg', 'min', 'med', '25th']:
      self._RunInternal(rep)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(GenerateBenchGraphs))
