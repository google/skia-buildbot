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
      '--skps', self._device_dirs.SKPDir(),
    ]

    match  = []

    if self._AnyMatch('Alex'):         # skia:2793
      args.extend(['--threads', '1'])

    if self._AnyMatch('Xoom'):         # skia:1699
      match.append('~WritePixels')

    if self._AnyMatch('GalaxyNexus'):  # skia:2900
      match.extend(['~filterindiabox', '~bleed'])

    # Though their GPUs are interesting, these don't test anything on
    # the CPU that other ARMv7+NEON bots don't test faster (N5).
    if self._AnyMatch('GalaxyNexus', 'Nexus10', 'Nexus7'):
      args.append('--nocpu')

    if match:
      args.append('--match')
      args.extend(match)

    self._flavor_utils.RunFlavoredCmd('dm', args)

    if self._AnyMatch('Valgrind'):     # skia:2789
      abandonGpuContext = list(args)
      abandonGpuContext.append('--abandonGpuContext')
      abandonGpuContext.append('--nocpu')
      self._flavor_utils.RunFlavoredCmd('dm', abandonGpuContext)

if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RunDM))
