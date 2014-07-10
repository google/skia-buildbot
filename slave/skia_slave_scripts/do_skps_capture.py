#!/usr/bin/env python
# Copyright (c) 2014 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


"""Run the webpages_playback automation script."""


import os
import posixpath
import sys

from build_step import BuildStep
from utils import old_gs_utils as gs_utils
from py.utils import shell_utils


class SKPsCapture(BuildStep):
  """BuildStep that captures the buildbot SKPs."""

  def __init__(self, timeout=10800, **kwargs):
    super(SKPsCapture, self).__init__(timeout=timeout, **kwargs)

  def _get_skp_version(self):
    """Find an unused SKP version."""
    current_skp_version = None
    version_file = os.path.join('third_party', 'skia', 'SKP_VERSION')
    with open(version_file) as f:
      current_skp_version = int(f.read().rstrip())

    # Find the first SKP version which has no uploaded SKPs.
    new_version = current_skp_version + 1
    while True:
      gs_path = posixpath.join(
          gs_utils.DEFAULT_DEST_GSBASE,
          self._storage_playback_dirs.PlaybackSkpDir(new_version))
      if not gs_utils.does_storage_object_exist(gs_path):
        return new_version
      new_version += 1

  def _Run(self):
    skp_version = self._get_skp_version()
    print 'SKP_VERSION=%d' % skp_version

    try:
      # Start Xvfb on the bot.
      shell_utils.run('sudo Xvfb :0 -screen 0 1280x1024x24 &', shell=True)
    except Exception:
      # It is ok if the above command fails, it just means that DISPLAY=:0
      # is already up.
      pass

    full_path_browser_executable = os.path.join(
        os.getcwd(), self._args['browser_executable'])

    upload_dir = 'playback_%d' % skp_version
    webpages_playback_cmd = [
      'python', os.path.join(os.path.dirname(os.path.realpath(__file__)),
                             'webpages_playback.py'),
      '--page_sets', self._args['page_sets'],
      '--browser_executable', full_path_browser_executable,
      '--non-interactive',
      '--upload_to_gs',
      '--alternate_upload_dir', upload_dir,
    ]

    try:
      shell_utils.run(webpages_playback_cmd)
    finally:
      # Clean up any leftover browser instances. This can happen if there are
      # telemetry crashes, processes are not always cleaned up appropriately by
      # the webpagereplay and telemetry frameworks.
      cleanup_cmd = [
        'pkill', '-9', '-f', full_path_browser_executable
      ]
      try:
        shell_utils.run(cleanup_cmd)
      except Exception:
        # Do not fail the build step if the cleanup command fails.
        pass


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(SKPsCapture))
