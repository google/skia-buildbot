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
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Debug_32',
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Debug_32_Trybot',
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_32',
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_32_Trybot',
        'Skia_Shuttle_Win7_Intel_Float_Debug_64',
        'Skia_Shuttle_Win7_Intel_Float_Debug_64_Trybot',
        'Skia_Shuttle_Win7_Intel_Float_Release_64',
        'Skia_Shuttle_Win7_Intel_Float_Release_64_Trybot',
        'Skia_Mac_Float_Debug_64',
        'Skia_Mac_Float_Debug_64_Trybot',
        'Skia_Mac_Float_Release_64',
        'Skia_Mac_Float_Release_64_Trybot',
        'Skia_MacMiniLion_Float_Debug_64',
        'Skia_MacMiniLion_Float_Debug_64_Trybot',
        'Skia_MacMiniLion_Float_Release_64',
        'Skia_MacMiniLion_Float_Release_64_Trybot',
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
