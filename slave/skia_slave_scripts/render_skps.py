#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia render_pictures executable. """

from build_step import BuildStep, BuildStepWarning
import os
import sys


JSON_SUMMARY_FILENAME_FORMATTER = 'renderskp-%s.json'
# TODO(epoger): Consider defining these render modes in a separate config file
# within the Skia repo, like we do with
# https://skia.googlesource.com/skia/+/master/tools/bench_pictures.cfg
# Or, maybe we should even share the specific render modes with bench_pictures?
# (Generally, we want to test the same code for both correctness and
# performance.)
DEFAULT_TILE_X = 256
DEFAULT_TILE_Y = 256
RENDER_MODES = {
    'defaults': [],
    'deferImageDecoding': ['--deferImageDecoding', '--useVolatileCache'],
    'grid': ['--bbh', 'grid', str(DEFAULT_TILE_X), str(DEFAULT_TILE_X) ],
    'rtree': ['--bbh', 'rtree' ],
}

# Keys we use within render_pictures's --descriptions argument.
# They are just for human consumption; they don't need to be kept in sync
# with values anywhere else.
DESCRIPTION__BUILDER = 'builder'
DESCRIPTION__RENDER_MODE = 'renderMode'


class RenderSKPs(BuildStep):

  def __init__(self, timeout=9600, no_output_timeout=9600, **kwargs):
    super(RenderSKPs, self).__init__(
      timeout=timeout, no_output_timeout=no_output_timeout, **kwargs)


  def DoRenderSKPs(self, args, render_mode_name, config='8888',
                   write_images=True):
    """Run render_pictures.

    Args:
      args: (list of strings) misc args to append to the command line
      render_mode_name: (string) human-readable description of this particular
          RenderSKPs run (perhaps "default", or "tiled"); used as part
          of the JSON summary filename, and also inserted within the file
      config: (string) which config to run in
      write_images: (boolean) whether to save the generated images (IGNORED)

    Raises:
      BuildStepWarning if there was a problem, but the step should keep going.
      Something else if there was a major problem and we should give up now.
    """
    json_summary_filename = JSON_SUMMARY_FILENAME_FORMATTER % render_mode_name
    json_expectations_devicepath = self._flavor_utils.DevicePathJoin(
        self._device_dirs.PlaybackExpectedSummariesDir(), json_summary_filename)
    if not self._flavor_utils.DevicePathExists(json_expectations_devicepath):
      raise BuildStepWarning('could not find JSON expectations file %s' %
                             json_expectations_devicepath)

    # TODO(stephana): We should probably start rendering whole images too, not
    # just tiles.
    cmd = [
        '--config', config,
        '--descriptions',
            '='.join([DESCRIPTION__BUILDER, self._builder_name]),
            '='.join([DESCRIPTION__RENDER_MODE, render_mode_name]),
        '--mode', 'tile', str(DEFAULT_TILE_X), str(DEFAULT_TILE_Y),
        '--readJsonSummaryPath', json_expectations_devicepath,
        '--readPath', self._device_dirs.SKPDir(),
        '--writeChecksumBasedFilenames',
        '--writeJsonSummaryPath', self._flavor_utils.DevicePathJoin(
            self._device_dirs.PlaybackActualSummariesDir(),
            json_summary_filename),
    ]
    if write_images:
      cmd.extend([
          '--mismatchPath', self._device_dirs.PlaybackActualImagesDir()])
    cmd.extend(args)

    if False:
      # For now, skip --validate on all builders, since it takes more time,
      # and at last check failed on Windows.
      if not os.name == 'nt':
        cmd.append('--validate')

    self._flavor_utils.RunFlavoredCmd('render_pictures', cmd)


  def _Run(self):
    exceptions = []
    for render_mode_name, args in sorted(RENDER_MODES.iteritems()):
      try:
        self.DoRenderSKPs(args=args, render_mode_name=render_mode_name)
      except BuildStepWarning, e:
        exceptions.append(e)
        print e
    if exceptions:
      raise BuildStepWarning('\nGot %d exceptions:\n%s' % (
          len(exceptions), '\n'.join([repr(e) for e in exceptions])))


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RenderSKPs))
