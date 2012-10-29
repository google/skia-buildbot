#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia bench executable. """

from build_step import BuildStep
from chromeos_build_step import ChromeOSBuildStep
from run_bench import BenchArgs
from run_bench import PreBench
from run_bench import RunBench
from utils import ssh_utils
import sys

def DoBench(executable, perf_data_dir, device_perf_dir, data_file,
            ssh_username, ssh_host, ssh_port, extra_args=None):
  """ Runs a ChromeOS benchmarking executable.

  executable: string indicating which program to run
  perf_data_dir: output path for performance data
  device_perf_dir: path on the ChromeOS device where performance data will be
      temporarily stored.
  data_file: string containing the device path to the perf data file
  ssh_username: username for 
  extra_args: list of any extra arguments to pass to the executable.
  """
  cmd_args = extra_args or []
  if perf_data_dir:
    PreBench(perf_data_dir)
    try:
      ssh_utils.RunSSH(ssh_username, ssh_host, ssh_port,
                       ['rm', '-rf', device_perf_dir])
    except:
      pass
    ssh_utils.RunSSH(ssh_username, ssh_host, ssh_port,
                     ['mkdir', '-p', device_perf_dir])
    cmd_args += BenchArgs(RunBench.BENCH_REPEAT_COUNT, data_file)
    ssh_utils.RunSSH(ssh_username, ssh_host, ssh_port,
                     [executable] + cmd_args)
    ssh_utils.GetSCP(perf_data_dir, data_file, ssh_username, ssh_host, ssh_port)
    ssh_utils.RunSSH(ssh_username, ssh_host, ssh_port,
                     ['rm', '-rf', device_perf_dir])
  else:
    ssh_utils.RunSSH(ssh_username, ssh_host, ssh_port,
                     [executable] + cmd_args)


class ChromeOSRunBench(RunBench, ChromeOSBuildStep):
  def _Run(self):
    data_file = self._BuildDataFile(self._device_dirs.PerfDir())
    DoBench(executable='skia_bench',
            perf_data_dir=self._perf_data_dir,
            device_perf_dir=self._device_dirs.PerfDir(),
            data_file=data_file,
            ssh_username=self._ssh_username,
            ssh_host=self._ssh_host,
            ssh_port=self._ssh_port)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSRunBench))