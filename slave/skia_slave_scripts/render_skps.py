#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pictures executable. """

from build_step import BuildStep
import os
import sys


DEFAULT_TILE_X = 256
DEFAULT_TILE_Y = 256
JSON_SUMMARY_BASENAME = 'renderskp-'


class RenderSKPs(BuildStep):
  def __init__(self, timeout=9600, no_output_timeout=9600, **kwargs):
    super(RenderSKPs, self).__init__(
      timeout=timeout, no_output_timeout=no_output_timeout, **kwargs)

  def DoRenderSKPs(self, args, config='8888', write_images=True,
                       json_summary_filename=None):
    """Run render_pictures.

    Args:
      args: (list of strings) misc args to append to the command line
      config: (string) which config to run in
      write_images: (boolean) whether to save the generated images (IGNORED)
      json_summary_filename: (string) name of file to write summary of actually-
          generated images into
    """
    # For now, don't run on Android, since it takes too long and we don't use
    # the results.
    if 'Android' in self._builder_name:
      return

    cmd = ['-r', self._device_dirs.SKPDir(), '--config', config,
           '--mode', 'tile', str(DEFAULT_TILE_X), str(DEFAULT_TILE_Y)]
    if json_summary_filename:
      cmd.extend(['--writeJsonSummaryPath', os.path.join(
          self._device_dirs.SKPOutDir(), json_summary_filename)])
    cmd.extend(args)

    if False:
      # For now, skip --validate and writing images on all builders, since they
      # take too long and we aren't making use of them.
      # Also skip --validate on Windows, where it is currently failing.
      if write_images:
        cmd.extend(['-w', self._device_dirs.SKPOutDir()])
      if not os.name == 'nt':
        cmd.append('--validate')
    self._flavor_utils.RunFlavoredCmd('render_pictures', cmd)

  def _Run(self):
    self.DoRenderSKPs(
        args=[],
        json_summary_filename=JSON_SUMMARY_BASENAME + 'defaults.json')
    self.DoRenderSKPs(
        args=['--bbh', 'grid', str(DEFAULT_TILE_X), str(DEFAULT_TILE_X),
              '--clone', '1'],
        json_summary_filename=JSON_SUMMARY_BASENAME + 'grid.json')
    self.DoRenderSKPs(
        args=['--bbh', 'rtree', '--clone', '2'],
        json_summary_filename=JSON_SUMMARY_BASENAME + 'rtree.json')
    self.DoRenderSKPs(
        args=['--deferImageDecoding', '--useVolatileCache'],
        json_summary_filename=JSON_SUMMARY_BASENAME + 'deferImageDecoding.json')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RenderSKPs))
