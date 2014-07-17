#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Run the Skia benchmarking executable."""

import os
import sys

from build_step import BuildStep
from utils import gclient_utils

import builder_name_schema

class RunNanobench(BuildStep):
  """A BuildStep that runs nanobench."""

  def __init__(self, timeout=9600, no_output_timeout=9600, **kwargs):
    super(RunNanobench, self).__init__(timeout=timeout,
                                       no_output_timeout=no_output_timeout,
                                       **kwargs)

  def _KeyParam(self):
    """Build a unique key from the builder name.

    E.g.:  os Mac10.6 model MacMini4.1 gpu GeForce320M arch x86

    This info is used by nanobench in its JSON output.
    """
    params = builder_name_schema.DictForBuilderName(self._builder_name)
    blacklist = ['configuration', 'role', 'is_trybot']
    # Don't include role (always Perf) or configuration (always Release).
    # TryBots can use the same exact key as they are uploaded to a different
    # location.
    #
    # It would be great to simplify this even further, but right now we have
    # two models for the same GPU (GalaxyNexus/NexusS for SGX540) and two
    # gpus for the same model (ShuttleA for GTX660/HD7770).
    for name in blacklist:
      if name in params:
        del params[name]
    keys = sorted(params.keys())
    return ' '.join([k + ' ' + params[k] for k in keys])

  def _JSONPath(self):
    git_timestamp = gclient_utils.GetGitRepoPOSIXTimestamp()
    return os.path.join(
            self._device_dirs.PerfDir(),
            'nanobench_%s_%s.json' % ( self._got_revision, git_timestamp))

  def _AnyMatch(self, *args):
    return any(arg in self._builder_name for arg in args)

  def _AllMatch(self, *args):
    return all(arg in self._builder_name for arg in args)

  def _Run(self):
    args = ['-i', self._device_dirs.ResourceDir()]
    if self._perf_data_dir:
      args.extend([
        '--outResultsFile', self._JSONPath(),
        '--key', self._KeyParam(),
        '--gitHash', self._got_revision,
        ])

    run_nanobench = True
    match  = []

    # Disable known problems.
    if self._AnyMatch('Android'):
      # Segfaults when run as GPU bench.  Very large texture?
      match.append('~blurroundrect')

    if self._AnyMatch('HD2000'):
      # GPU benches seem to hang on HD2000.  Not sure why.
      args.append('--nogpu')

    if self._AnyMatch('Nexus7'):
      # Crashes in GPU mode.
      match.append('~draw_stroke')

    if match:
      args.append('--match')
      args.extend(match)

    if run_nanobench:
      self._flavor_utils.RunFlavoredCmd('nanobench', args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunNanobench))
