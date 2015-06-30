# Copyright 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

# This file has been copied from
# https://chromium.googlesource.com/chromium/src/+/master/tools/perf/benchmarks/rasterize_and_record_micro.py
# and modified locally to support CT pagesets. Hopefully one day this file
# will live in telemetry codebase instead.

from benchmarks import skpicture_printer
from measurements import rasterize_and_record_micro
import page_sets
from telemetry import benchmark


class _RasterizeAndRecordMicro(benchmark.Benchmark):
  @classmethod
  def AddBenchmarkCommandLineArgs(cls, parser):
    parser.add_option('--start-wait-time', type='float',
                      default=2,
                      help='Wait time before the benchmark is started '
                      '(must be long enought to load all content)')
    parser.add_option('--rasterize-repeat', type='int',
                      default=100,
                      help='Repeat each raster this many times. Increase '
                      'this value to reduce variance.')
    parser.add_option('--record-repeat', type='int',
                      default=100,
                      help='Repeat each record this many times. Increase '
                      'this value to reduce variance.')
    parser.add_option('--timeout', type='int',
                      default=120,
                      help='The length of time to wait for the micro '
                      'benchmark to finish, expressed in seconds.')
    parser.add_option('--report-detailed-results',
                      action='store_true',
                      help='Whether to report additional detailed results.')
    parser.add_option('--page-set-name',  action='store', type='string')
    parser.add_option('--page-set-base-dir', action='store', type='string')

  def CreatePageTest(self, options):
    return rasterize_and_record_micro.RasterizeAndRecordMicro(
        options.start_wait_time, options.rasterize_repeat,
        options.record_repeat, options.timeout, options.report_detailed_results)


@benchmark.Disabled
class RasterizeAndRecordMicroCTPages(_RasterizeAndRecordMicro):
  test = rasterize_and_record_micro.RasterizeAndRecordMicro

  @classmethod
  def ProcessCommandLineArgs(cls, parser, args):
    if not args.page_set_name:
      parser.error('Please specify --page-set-name')
    if not args.page_set_base_dir:
      parser.error('Please specify --page-set-base-dir')

  def CreateStorySet(self, options):
    page_set_class = skpicture_printer._MatchPageSetName(
        options.page_set_name, options.page_set_base_dir)
    return page_set_class()
