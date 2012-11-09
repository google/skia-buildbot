#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Runs the bench_pictures executable on skp files from archived webpages.

This module can be run from the command-line like this:

cd buildbot/third_party/chromium_buildbot/slave/\
Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64/build/trunk

PYTHONPATH=../../../../site_config \
python ../../../../../../slave/skia_slave_scripts/bench_webpage_pictures.py \
--configuration "Debug" --target_platform "" --revision 6444 \
--autogen_svn_baseurl "" --make_flags "" --test_args "" --gm_args "" \
--bench_args "" --num_cores 8 --perf_output_basedir "../../../../perfdata" \
--builder_name Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64 \
--got_revision 6444 --gm_image_subdir "" \
--dest_gsbase ""

"""

from build_step import BuildStep
from bench_pictures import BenchPictures
import sys


class BenchWebpagePictures(BenchPictures):
  """Runs the bench_pictures executable on skp files from archived webpages."""

  def _GetSkpDir(self):
    """Points to the local playback skp directory."""
    return self._local_playback_dirs.PlaybackSkpDir()

  def _GetPerfDataDir(self):
    """Points to the local playback perf data directory."""
    return self._local_playback_dirs.PlaybackPerfDataDir()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(BenchWebpagePictures))
