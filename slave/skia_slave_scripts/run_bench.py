#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Run the Skia benchmarking executable."""

import os
import sys

from build_step import BuildStep

GIT = 'git.bat' if os.name == 'nt' else 'git'
GIT_SVN_ID_MATCH_STR = r'git-svn-id: http://skia.googlecode.com/svn/trunk@(\d+)'


def BenchArgs(data_file):
  """Builds a list containing arguments to pass to bench.

  Args:
    data_file: filepath to store the log output
  Returns:
    list containing arguments to pass to bench
  """
  return ['--timers', 'wcg', '--logFile', data_file]

# Device name -> extra arguments
EXTRA_ARGS = {
    'GalaxyNexus': ['--match', '~DeferredSurfaceCopy'],  # Crash: skbug.com/1687
    'Nexus4': ['--config', 'defaults', 'MSAA4'],
    'NexusS': ['--match', '~DeferredSurfaceCopy'],       # Crash: skbug.com/1687
    'Valgrind': ['--runOnce', 'true', '--config', '8888', 'GPU',
                 'NONRENDERING'],
}


class RunBench(BuildStep):
  """A BuildStep that runs bench."""

  def __init__(self, timeout=9600, no_output_timeout=9600, **kwargs):
    super(RunBench, self).__init__(timeout=timeout,
                                   no_output_timeout=no_output_timeout,
                                   **kwargs)

  def _BuildDataFile(self):
    return os.path.join(self._device_dirs.PerfDir(),
                        'bench_%s_data' % self._got_revision)

  def _Run(self):
    args = ['-i', self._device_dirs.ResourceDir()]
    if self._perf_data_dir:
      args.extend(BenchArgs(self._BuildDataFile()))
    for builder_name_fragment in EXTRA_ARGS:
      if builder_name_fragment in self._builder_name:
        args.extend(EXTRA_ARGS[builder_name_fragment])
    self._flavor_utils.RunFlavoredCmd('bench', args + self._bench_args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunBench))
