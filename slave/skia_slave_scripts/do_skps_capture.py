#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run the webpages_playback automation script."""


import os
import sys

from build_step import BuildStep
from utils import shell_utils


class SKPsCapture(BuildStep):
  """BuildStep that captures the buildbot SKPs."""

  def __init__(self, timeout=10800, **kwargs):
    super(SKPsCapture, self).__init__(timeout=timeout, **kwargs)

  def _Run(self):
    webpages_playback_cmd = [
      'python', os.path.join(os.path.dirname(os.path.realpath(__file__)),
                             'webpages_playback.py'),
      '--page_sets', self._args['page_sets'],
      '--skia_tools', self._args['skia_tools'],
      '--browser_executable', self._args['browser_executable'],
      '--non-interactive'
    ]
    if not self._is_try:
      webpages_playback_cmd.append('--upload_to_gs')
    shell_utils.run(webpages_playback_cmd)

    # Clean up any leftover browser instances. This can happen if there are
    # telemetry crashes, processes are not always cleaned up appropriately by
    # the webpagereplay and telemetry frameworks.
    cleanup_cmd = [
      'pkill', '-9', '-f', os.path.join(os.getcwd(),
                                        self._args['browser_executable'])
    ]
    shell_utils.run(cleanup_cmd)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(SKPsCapture))
