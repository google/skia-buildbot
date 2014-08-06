#!/usr/bin/env python
# Copyright (c) 2013 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.


""" Run the Skia render_pdfs executable. """


from build_step import BuildStep, BuildStepWarning
from py.utils.shell_utils import CommandFailedException
import sys


class RenderPdfs(BuildStep):
  def _Run(self):
    try:
      self._flavor_utils.RunFlavoredCmd('render_pdfs',
                                        [self._device_dirs.SKPDir()])
    except CommandFailedException, e:
      if ('Nexus4' in self._builder_name or
          'NexusS' in self._builder_name or
          'Xoom'   in self._builder_name):
        raise BuildStepWarning('skia:2743 RenderPdfs is known to fail on %s: %s'
                               % (self._builder_name, str(e)))
      else:
        raise e


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(RenderPdfs))
