#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia benchmarking executable. """

from build_step import BuildStep
from utils import shell_utils
import os
import re
import sys


GIT = 'git.bat' if os.name == 'nt' else 'git'
GIT_SVN_ID_MATCH_STR = 'git-svn-id: http://skia.googlecode.com/svn/trunk@(\d+)'


def BenchArgs(data_file):
  """ Builds a list containing arguments to pass to bench.

  data_file: filepath to store the log output
  """
  return ['--timers', 'wcg', '--logFile', data_file]


def GetSvnRevision(commit_hash):
  output = shell_utils.Bash([GIT, 'show', '-s', commit_hash],
                            echo=False, log_in_real_time=False)
  results = re.findall(GIT_SVN_ID_MATCH_STR, output)
  if results:
    return results[0]
  else:
    raise Exception('No git-svn-id found for %s\nOutput:\n%s' % (commit_hash,
                                                                 output))


class RunBench(BuildStep):
  def __init__(self, timeout=9600, no_output_timeout=9600, **kwargs):
    super(RunBench, self).__init__(timeout=timeout,
                                   no_output_timeout=no_output_timeout,
                                   **kwargs)

  def _BuildDataFile(self):
    return os.path.join(self._device_dirs.PerfDir(),
                        'bench_r%s_data' % GetSvnRevision(self._got_revision))

  def _Run(self):
    args = ['-i', self._device_dirs.ResourceDir()]
    if self._perf_data_dir:
      args.extend(BenchArgs(self._BuildDataFile()))
    if 'Nexus4' in self._builder_name:
      args.extend(['--config', 'defaults', 'MSAA4'])
    self._flavor_utils.RunFlavoredCmd('bench', args + self._bench_args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunBench))
