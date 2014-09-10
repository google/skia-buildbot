#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Step to run after the render steps. """

from build_step import BuildStep
import sys


class PostRender(BuildStep):
  def _RunAfterGenerateGMs(self):
    self._flavor_utils.CopyDirectoryContentsToHost(
        self._flavor_utils.DevicePathJoin(self._device_dirs.GMActualDir(),
                                          self._builder_name),
                                                   self._gm_actual_dir)

  def _RunAfterDM(self):
    self._flavor_utils.CopyDirectoryContentsToHost(self._device_dirs.DMDir(),
                                                   self._dm_dir)

  def _RunAfterRunDecodingTests(self):
    self._flavor_utils.CopyDirectoryContentsToHost(
        self._device_dirs.SKImageOutDir(), self._skimage_out_dir)

  def _RunAfterRenderSKPs(self):
    self._flavor_utils.CopyDirectoryContentsToHost(
        self._device_dirs.PlaybackActualImagesDir(),
        self.playback_actual_images_dir)
    self._flavor_utils.CopyDirectoryContentsToHost(
        self._device_dirs.PlaybackActualSummariesDir(),
        self.playback_actual_summaries_dir)


  def _Run(self):
    self._RunAfterGenerateGMs()
    self._RunAfterDM()
    self._RunAfterRunDecodingTests()
    self._RunAfterRenderSKPs()


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(PostRender))
