#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Step to run before the benchmarking steps. """

from build_step import BuildStep
from chromeos_build_step import ChromeOSBuildStep
from prebench import PreBench
from utils import ssh_utils
import sys


class ChromeOSPreBench(ChromeOSBuildStep, PreBench):
  def _Run(self):
    super(ChromeOSPreBench, self)._Run()

    if self._perf_data_dir:
      try:
        ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                         ['rm', '-rf', self._device_dirs.PerfDir()])
      except Exception:
        pass
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                       ['mkdir', '-p', self._device_dirs.PerfDir()])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSPreBench))