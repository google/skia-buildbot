#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Step to run after the benchmarking steps. """

from build_step import BuildStep
from chromeos_build_step import ChromeOSBuildStep
from postbench import PostBench
from utils import ssh_utils
import posixpath
import sys


class ChromeOSPostBench(ChromeOSBuildStep, PostBench):
  def _Run(self):
    super(ChromeOSPostBench, self)._Run()

    if self._perf_data_dir:
      ssh_utils.GetSCP(self._perf_data_dir,
                       posixpath.join(self._device_dirs.PerfDir(), '*'),
                       self._ssh_username, self._ssh_host, self._ssh_port,
                       recurse=True)
      ssh_utils.RunSSH(self._ssh_username, self._ssh_host, self._ssh_port,
                       ['rm', '-rf', self._device_dirs.PerfDir()])


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSPostBench))