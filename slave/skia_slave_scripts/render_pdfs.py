#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Run the Skia render_pdfs executable. """


from build_step import BuildStep
import sys

class RenderPdfs(BuildStep):
  def _Run(self):
    args = ['--inputPaths', self._device_dirs.SKPDir()]
    if ('Nexus4' in self._builder_name or
        'NexusS' in self._builder_name or
        'Xoom'   in self._builder_name):
      # On these devices, these SKPs usually make render_pdfs run out of memory.
      args.extend(['--match', '~tabl_mozilla', '~tabl_nytimes'])
    self._flavor_utils.RunFlavoredCmd('render_pdfs', args)


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RenderPdfs))
