#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Run the Skia DM executable. """

from build_step import BuildStep
import builder_name_schema
import sys

class RunDM(BuildStep):
  def __init__(self, *args, **kwargs):
    super(RunDM, self).__init__(*args, **kwargs)
    # Valgrind is slow.
    if self._AnyMatch('Valgrind'):
      self.timeout *= 5

  def _AnyMatch(self, *args):
    return any(arg in self._builder_name for arg in args)

  def _KeyParams(self):
    """Build a unique key from the builder name (as a list).

    E.g.  arch x86 gpu GeForce320M mode MacMini4.1 os Mac10.6
    """
    # Don't bother to include role, which is always Test.
    # TryBots are uploaded elsewhere so they can use the same key.
    blacklist = ['role', 'is_trybot']

    params = builder_name_schema.DictForBuilderName(self._builder_name)
    flat = []
    for k in sorted(params.keys()):
      if k not in blacklist:
        flat.append(k)
        flat.append(params[k])
    return flat

  def _Run(self):
    args = [
      '--verbose',
      '--resourcePath', self._device_dirs.ResourceDir(),
      '--skps',         self._device_dirs.SKPDir(),
      '--writePath',    self._device_dirs.DMDir(),
      '--nameByHash',
      '--properties',  'gitHash',      self._got_revision,
                       'build_number', str(self._build_number),
    ]
    args.append('--key')
    args.extend(self._KeyParams())

    match  = []

    if self._AnyMatch('Alex'):         # skia:2793
      args.extend(['--threads', '1'])

    if self._AnyMatch('Xoom'):         # skia:1699
      match.append('~WritePixels')

    if self._AnyMatch('GalaxyNexus'):  # skia:2900
      match.extend(['~filterindiabox', '~bleed'])

    if self._AnyMatch('Venue8'):       # skia:2922
      match.append('~imagealphathreshold')

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
