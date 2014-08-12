#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Run the Skia DM executable. """

from build_step import BuildStep
import sys

class RunDM(BuildStep):
  def __init__(self, *args, **kwargs):
    super(RunDM, self).__init__(*args, **kwargs)
    # Valgrind is slow.
    if self._AnyMatch('Valgrind'):
      self.timeout *= 5

  def _AnyMatch(self, *args):
    return any(arg in self._builder_name for arg in args)

  def _Run(self):
    args = [
      '--verbose',
      '--resourcePath', self._device_dirs.ResourceDir(),
    ]

    match  = []

    if self._AnyMatch('Alex'):
      # This machine looks to be running out of heap.
      # Running with fewer threads may help.
      args.extend(['--threads', '1'])

    if self._AnyMatch('Android'):
      match.append('~giantbitmap')

    if self._AnyMatch('Tegra'):
      match.append('~downsamplebitmap_text')

    if self._AnyMatch('Xoom'):
      match.append('~WritePixels')  # skia:1699

    # Nexus S and Galaxy Nexus are still crashing.
    # Maybe the GPU's the problem?
    if self._AnyMatch('SGX540'):
      args.append('--nogpu')

    if match:
      args.append('--match')
      args.extend(match)

    self._flavor_utils.RunFlavoredCmd('dm', args)

    # See skia:2789
    if self._AnyMatch('Valgrind'):
      abandonGpuContext = list(args)
      abandonGpuContext.append('--abandonGpuContext')
      abandonGpuContext.append('--nocpu')
      self._flavor_utils.RunFlavoredCmd('dm', abandonGpuContext)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunDM))
