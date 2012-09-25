#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench executable. """

from android_build_step import AndroidBuildStep
from build_step import BuildStep
from run_bench import BenchArgs
from run_bench import PreBench
from run_bench import RunBench
from utils import misc
import sys

def DoBench(serial, executable, perf_data_dir, adb_perf_dir, app_data_file,
            adb_data_file, extra_args=None):
  """ Runs an Android benchmarking executable.
  
  serial: string indicating serial number of the Android device to target
  executable: string indicating which program to run
  perf_data_dir: output path for performance data
  adb_perf_dir: path on the Android device where performance data will be
      temporarily stored.
  adb_data_file: string containing the ADB path to the perf data file
  app_data_file: string containing the Android app path to the perf data file
  extra_args: list of any extra arguments to pass to the executable.
  """
  cmd_args = extra_args or []
  if perf_data_dir:
    PreBench(perf_data_dir)
    try:
      misc.RunADB(serial, ['shell', 'rm', '-r', adb_perf_dir])
    except:
      pass
    misc.RunADB(serial, ['shell', 'mkdir', '-p', adb_perf_dir])
    cmd_args += BenchArgs(RunBench.BENCH_REPEAT_COUNT, app_data_file)
    misc.Run(serial, executable, arguments=cmd_args)
    misc.RunADB(serial, ['pull', adb_data_file, perf_data_dir])
    misc.RunADB(serial, ['shell', 'rm', '-r', adb_perf_dir])
  else:
    misc.Run(serial, executable, arguments=cmd_args)

class AndroidRunBench(RunBench, AndroidBuildStep):
  def _Run(self, args):
    app_data_file = self._BuildDataFile(self._app_dirs.PerfDir())
    adb_data_file = self._BuildDataFile(self._adb_dirs.PerfDir())
    DoBench(serial=self._serial,
            executable='bench',
            perf_data_dir=self._perf_data_dir,
            adb_perf_dir=self._adb_dirs.PerfDir(),
            app_data_file=app_data_file,
            adb_data_file=adb_data_file)

if '__main__' == __name__:
  sys.exit(BuildStep.Run(AndroidRunBench))