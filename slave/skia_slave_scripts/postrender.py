#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Step to run after the render steps. """

from build_step import BuildStep
import sys


class PostRender(BuildStep):
  def _Run(self):
    self.CopyDirectoryContentsToHost(self.DevicePathJoin(
                                         self._device_dirs.GMActualDir(),
                                         self._gm_image_subdir),
                                     self._gm_actual_dir)
    self.CopyDirectoryContentsToHost(self._device_dirs.SKPOutDir(),
        self._local_playback_dirs.PlaybackGmActualDir())
    self.CopyDirectoryContentsToHost(self._device_dirs.SKImageOutDir(),
                                     self._skimage_out_dir)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(PostRender))