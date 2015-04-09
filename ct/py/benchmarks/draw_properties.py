# Copyright 2015 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This file has been copied from
# https://chromium.googlesource.com/chromium/src/+/master/tools/perf/benchmarks/draw_properties.py
# and modified locally to support CT pagesets. Hopefully one day this file
# will live in telemetry codebase instead.

from benchmarks import silk_flags
from benchmarks import skpicture_printer
from measurements import draw_properties
import page_sets
from telemetry import benchmark


@benchmark.Disabled
class DrawPropertiesCTPages(benchmark.Benchmark):
  test = draw_properties.DrawProperties

  @classmethod
  def AddBenchmarkCommandLineArgs(cls, parser):
    parser.add_option('--page-set-name',  action='store', type='string')
    parser.add_option('--page-set-base-dir', action='store', type='string')

  @classmethod
  def ProcessCommandLineArgs(cls, parser, args):
    if not args.page_set_name:
      parser.error('Please specify --page-set-name')
    if not args.page_set_base_dir:
      parser.error('Please specify --page-set-base-dir')

  def CreatePageSet(self, options):
    page_set_class = skpicture_printer._MatchPageSetName(
        options.page_set_name, options.page_set_base_dir)
    return page_set_class()

