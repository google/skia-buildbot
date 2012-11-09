#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

"""Upload benchmark performance data results from archived webpages.

This module can be run from the command-line like this:

cd buildbot/third_party/chromium_buildbot/slave/\
Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64/build/trunk

PYTHONPATH=../../../../site_config:\
../../../../scripts \
python ../../../../../../slave/skia_slave_scripts/\
upload_webpage_picture_bench_results.py \
--configuration "" --target_platform "" --revision 0 \
--autogen_svn_baseurl "" --make_flags "" --test_args "" --gm_args "" \
--bench_args "" --num_cores 8 --perf_output_basedir "../../../../perfdata" \
--builder_name Skia_Shuttle_Ubuntu12_ATI5770_Float_Release_64 \
--got_revision 0 --gm_image_subdir "" \
--dest_gsbase "gs://rmistry"

"""

from build_step import BuildStep
from upload_bench_results import UploadBenchResults

import sys


class UploadWebpagePictureBenchResults(UploadBenchResults):
  """Upload benchmark performance data results from archived webpages."""

  def __init__(self, args, attempts=5):
    """Create an instance of UploadWebpagePictureBenchResults."""
    super(UploadBenchResults, self).__init__(args, attempts)

  def _GetPerfDataDir(self):
    """Points to the local playback perf data directory."""
    return self._local_playback_dirs.PlaybackPerfDataDir()

  def _GetBucketSubdir(self):
    """Results the playback perf data bucket."""
    return self._storage_playback_dirs.PlaybackPerfDataDir()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadWebpagePictureBenchResults))

