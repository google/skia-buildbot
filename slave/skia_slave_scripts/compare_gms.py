#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compare the generated GM images to the baselines """

from utils import shell_utils
from build_step import BuildStep, BuildStepWarning
import sys


class CompareGMs(BuildStep):
  def _Run(self):
    cmd = [self._PathToBinary('skdiff'),
           '--listfilenames',
           '--nodiffs',
           '--nomatch', 'README',
           '--failonresult', 'DifferentPixels',
           '--failonresult', 'DifferentSizes',
           '--failonresult', 'Unknown',
           '--failonstatus', 'CouldNotDecode,CouldNotRead', 'any',
           '--failonstatus', 'any', 'CouldNotDecode,CouldNotRead',
           self._gm_expected_dir,
           self._gm_actual_dir,
           ]

    # Temporary list of builders who are allowed to fail this step without the
    # bot turning red.
    may_fail_with_warning = [
        'Test-Ubuntu12-ShuttleA-ATI5770-x86-Debug',
        'Test-Ubuntu12-ShuttleA-ATI5770-x86-Debug-Trybot',
        'Test-Ubuntu12-ShuttleA-ATI5770-x86-Release',
        'Test-Ubuntu12-ShuttleA-ATI5770-x86-Release-Trybot',
        'Test-Win7-ShuttleA-HD2000-x86_64-Debug',
        'Test-Win7-ShuttleA-HD2000-x86_64-Debug-Trybot',
        'Test-Win7-ShuttleA-HD2000-x86_64-Release',
        'Test-Win7-ShuttleA-HD2000-x86_64-Release-Trybot',
        'Test-Mac10.6-MacMini4.1-GeForce320M-x86_64-Debug',
        'Test-Mac10.6-MacMini4.1-GeForce320M-x86_64-Debug-Trybot',
        'Test-Mac10.6-MacMini4.1-GeForce320M-x86_64-Release',
        'Test-Mac10.6-MacMini4.1-GeForce320M-x86_64-Release-Trybot',
        'Test-Mac10.7-MacMini4.1-GeForce320M-x86_64-Debug',
        'Test-Mac10.7-MacMini4.1-GeForce320M-x86_64-Debug-Trybot',
        'Test-Mac10.7-MacMini4.1-GeForce320M-x86_64-Release',
        'Test-Mac10.7-MacMini4.1-GeForce320M-x86_64-Release-Trybot',
        ]
    try:
      shell_utils.Bash(cmd)
    except Exception as e:
      if self._builder_name in may_fail_with_warning:
        raise BuildStepWarning(e)
      else:
        raise


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CompareGMs))
