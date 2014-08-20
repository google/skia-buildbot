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

  def _KeyParams(self):
    """Build a unique key from the builder name (as a list).

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
    flat = []
    for k in sorted(params.keys()):
      flat.append(k)
      flat.append(params[k])
    return flat

  def _JSONPath(self):
    git_timestamp = gclient_utils.GetGitRepoPOSIXTimestamp()
    return os.path.join(
            self._device_dirs.PerfDir(),
            'nanobench_%s_%s.json' % ( self._got_revision, git_timestamp))

  def _AnyMatch(self, *args):
    return any(arg in self._builder_name for arg in args)

  def _Run(self):
    args = ['-i', self._device_dirs.ResourceDir(),
            '--skps', self._device_dirs.SKPDir(),
            '--scales', '1.0', '1.1']
    if self._AnyMatch('Valgrind'):
      args.extend(['--loops', '1'])  # Don't care about performance on Valgrind.
    elif self._perf_data_dir:
      args.extend(['--outResultsFile', self._JSONPath()])
      args.append('--key')
      args.extend(self._KeyParams())
      args.append('--properties')
      args.extend(['gitHash', self._got_revision,
                   'build_number', str(self._build_number)])

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
      # Fatally overload the driver.
      match.extend(['~path_fill_big_triangle', '~lines_0'])

    if self._AnyMatch('Xoom'):
      match.append('~patch_grid')  # skia:2847

    if match:
      args.append('--match')
      args.extend(match)

    self._flavor_utils.RunFlavoredCmd('nanobench', args)

    # See skia:2789
    if self._AnyMatch('Valgrind'):
      abandonGpuContext = list(args)
      abandonGpuContext.append('--abandonGpuContext')
      abandonGpuContext.append('--nocpu')
      self._flavor_utils.RunFlavoredCmd('nanobench', abandonGpuContext)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunNanobench))
