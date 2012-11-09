#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Compares the GM images from archived webpages to the baselines.

This module can be run from the command-line like this:

cd buildbot/third_party/chromium_buildbot/slave/\
Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64/build/trunk

PYTHONPATH=../../../../site_config \
python ../../../../../../slave/skia_slave_scripts/compare_webpage_gms.py \
--configuration "Debug" --target_platform "" --revision 0 \
--autogen_svn_baseurl "" --make_flags "" --test_args "" --gm_args "" \
--bench_args "" --num_cores 8 --perf_output_basedir "" \
--builder_name Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64 \
--got_revision 0 --gm_image_subdir base-shuttle_ubuntu12_ati5770 \
--dest_gsbase ""

"""

from utils import misc
from build_step import BuildStep, BuildStepWarning
import sys


class CompareWebpageGMs(BuildStep):
  def _Run(self):
    cmd = [self._PathToBinary('skdiff'),
           '--listfilenames',
           '--nodiffs',
           '--nomatch', 'README',
           '--failonresult', 'DifferentPixels',
           '--failonresult', 'DifferentSizes',
           '--failonresult', 'DifferentOther',
           '--failonresult', 'Unknown',
           self._local_playback_dirs.PlaybackGmExpectedDir(),
           self._local_playback_dirs.PlaybackGmActualDir(),
           ]

    # Temporary list of builders who are allowed to fail this step without the
    # bot turning red.
    may_fail_with_warning = [
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Debug_32',
        'Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_32',
        'Skia_Shuttle_Win7_Intel_Float_Debug_64',
        'Skia_Shuttle_Win7_Intel_Float_Release_64',
        'Skia_Mac_Float_Debug_64',
        'Skia_Mac_Float_Release_64',
        'Skia_MacMiniLion_Float_Debug_64',
        'Skia_MacMiniLion_Float_Release_64'
        ]

    try:
      misc.Bash(cmd)
    except Exception as e:
      if self._builder_name in may_fail_with_warning:
        raise BuildStepWarning(e)
      else:
        raise


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(CompareWebpageGMs))
