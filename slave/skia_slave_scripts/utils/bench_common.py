#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Common utilities and constants used by the benchmarking build steps. """

import os

BENCH_GRAPH_NUM_REVISIONS = 15
BENCH_GRAPH_X = 1024
BENCH_GRAPH_Y = 768

# TODO: This is a workaround for
# https://code.google.com/p/skia/issues/detail?id=685
# ('gsutil upload fails with "BotoServerError: 500 Internal Server Error", but
# only for certain destination filenames').
# Modify this value to generate graphs with a new filename that will upload
# successfully, when uploading the old filename starts to fail.
GRAPH_PATH_MODIFIER = '2'

def GraphFilePath(perf_graphs_dir, builder_name, representation='default'):
  return os.path.join(perf_graphs_dir, 'graph-%s-%s-%s.xhtml' % (
      builder_name, representation, GRAPH_PATH_MODIFIER))
