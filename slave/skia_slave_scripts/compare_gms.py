#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Compare the generated GM images to the baselines """

# System-level imports
import os
import sys

from build_step import BuildStep, BuildStepWarning
from utils import misc, shell_utils
import run_gm

class CompareGMs(BuildStep):
  def _Run(self):
    json_summary_path = misc.GetAbsPath(os.path.join(
        self._gm_actual_dir, run_gm.JSON_SUMMARY_FILENAME))

    # TODO(epoger): It would be better to call the Python code in
    # display_json_results.py directly, rather than going through the
    # shell and launching another Python interpreter.  See
    # https://code.google.com/p/skia/issues/detail?id=1298 ('buildbots:
    # augment slave-side PYTHON_PATH, so slave-side scripts can easily call
    # Python code in Skia trunk')
    cmd = ['python',
           os.path.join('gm', 'display_json_results.py'),
           json_summary_path,
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
