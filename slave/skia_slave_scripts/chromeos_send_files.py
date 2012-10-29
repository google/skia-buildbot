#!/usr/bin/env python
# Copyright (c) 2012 The Chromium Authors. All rights reserved.
# Use of this source code is governed by a BSD-style license that can be
# found in the LICENSE file.

""" Run the Skia tests executable. """

from build_step import BuildStep
from chromeos_build_step import ChromeOSBuildStep
from utils import ssh_utils
import sys


class ChromeOSSendFiles(ChromeOSBuildStep):
  def _PutSCP(self, executable):
    ssh_utils.PutSCP(local_path=self._PathToBinary(executable),
                     remote_path='/usr/local/bin/skia_%s' % executable,
                     username=self._ssh_username,
                     host=self._ssh_host,
                     port=self._ssh_port)

  def _Run(self):
    self._PutSCP('tests')
    self._PutSCP('gm')
    self._PutSCP('render_pictures')
    self._PutSCP('bench')
    self._PutSCP('bench_pictures')


if '__main__' == __name__:
  sys.exit(BuildStep.RunBuildStep(ChromeOSSendFiles))