#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Upload new bench expectations for this bot."""


import os
import shutil
import sys
import time

from build_step import BuildStep
from utils import misc


class UploadBenchExpectations(BuildStep):
  """Takes the perf bench expectation file created from
  GenerateBenchExpectations, updates the corresponding file in Skia repo, and
  commits the change.
  """

  def _Run(self):
    dst_dir = os.path.join(os.getcwd(), 'expectations/bench')
    commit_msg = """bench rebase after %s (SkipBuildbotRuns)

This CL was created by Skia bots in UpdatePerfBaselines.

Bypassing commit queue trybots:
NOTRY=true""" % self._got_revision[:7]
    with misc.GitBranch('tmp_perf_rebase_branch_%s' % time.time(), commit_msg,
                        upload=True, commit_queue=False):
      for item in os.listdir(self._perf_range_input_dir):
        src_file = os.path.join(self._perf_range_input_dir, item)
        if os.path.isfile(src_file):
          shutil.copy(src_file, dst_dir)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(UploadBenchExpectations))
